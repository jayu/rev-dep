/*eslint-disable @typescript-eslint/no-var-requires */

const node_path = require('path')
const fs = require('fs')
const parser = require('@babel/parser')
const template = require('@babel/template').default
import { findTsConfig } from '../lib/utils'

const SKIP = Symbol('SKIP')

module.exports = function plugin({ types }, { tsConfigPath = findTsConfig() }) {
  const tsConfig = require(tsConfigPath)
  const aliases = tsConfig.compilerOptions.paths
  const aliasesKeys = Object.keys(aliases)
  const aliasesRegexes = Object.keys(aliases).map((alias) => {
    return new RegExp(`^${alias.replace('*', '(.)+')}$`)
  })

  const cache = new Map()

  const getFile = (original, paths) => {
    if (paths.length === 0) {
      throw new Error('Cannot resolve import ' + original)
    }

    const path = paths[0]
    try {
      return [path, fs.readFileSync(path).toString()]
    } catch (e) {
      return getFile(original, paths.slice(1))
    }
  }

  const isPathRelativeOrAliased = (path) => {
    const aliasRegexIdx = aliasesRegexes.findIndex((aliasRegex) =>
      aliasRegex.test(path)
    )

    const isRelative = path.startsWith('./') || path.startsWith('../')

    return aliasRegexIdx > -1 || isRelative
  }

  const cacheKey = (identifier, filePath) => `${identifier}-${filePath}`

  const lookup = (identifier, filePath, cwd) => {
    const cached = cache.get(cacheKey(identifier, filePath))

    if (cached) {
      return cached
    }

    const withExtension = /(\.ts|\.tsx)$/.test(filePath)
      ? [filePath]
      : [
          `${filePath}.ts`,
          `${filePath}.tsx`,
          `${filePath}/index.ts`,
          `${filePath}/index.tsx`,
          `${filePath}.js`,
          `${filePath}.jsx`,
          `${filePath}/index.js`,
          `${filePath}/index.jsx`
        ]

    const [resolvedFilePath, file] = getFile(filePath, withExtension)

    const ast = parser.parse(file, {
      sourceType: 'module',
      plugins: [
        'jsx',
        'typescript',
        'objectRestSpread',
        'classProperties',
        'asyncGenerators',
        'decorators-legacy'
      ]
    })

    /**
     * {
     *  identifier?: string,
     *  source: string
     * }
     */
    const toLookup = []
    let resolvedAs = null

    ast.program.body.forEach((declaration) => {
      if (resolvedAs === null) {
        if (types.isExportNamedDeclaration(declaration)) {
          if (types.isVariableDeclaration(declaration.declaration)) {
            const hasIdentifier = declaration.declaration.declarations.find(
              (declarator) => {
                return declarator.id.name === identifier
              }
            )

            if (hasIdentifier) {
              resolvedAs = {
                type: 'named',
                identifier,
                source: filePath
              }
            }
          } else if (
            types.isFunctionDeclaration(declaration.declaration) ||
            types.isClassDeclaration(declaration.declaration)
          ) {
            if (declaration.declaration.id.name === identifier) {
              resolvedAs = {
                type: 'named',
                identifier,
                source: filePath
              }
            }
          } else {
            const source = declaration.source?.value

            declaration.specifiers.forEach((specifier) => {
              if (types.isExportSpecifier(specifier)) {
                if (specifier.exported.name === identifier) {
                  if (specifier.local.name === 'default' && source) {
                    resolvedAs = {
                      type: 'default',
                      identifier,
                      source: getModulePath(source, resolvedFilePath, cwd)
                    }
                  } else if (source === undefined) {
                    resolvedAs = {
                      type: 'named',
                      identifier,
                      source: filePath
                    }
                  } else if (isPathRelativeOrAliased(source)) {
                    toLookup.push({
                      identifier: specifier.exported.local,
                      source: getModulePath(source, resolvedFilePath, cwd)
                    })
                  }
                }
              }
            })
          }
        } else if (
          types.isExportAllDeclaration(declaration) &&
          isPathRelativeOrAliased(declaration.source.value)
        ) {
          toLookup.push({
            identifier,
            source: getModulePath(
              declaration.source.value,
              resolvedFilePath,
              cwd
            )
          })
        }
      }
    })

    if (resolvedAs) {
      return resolvedAs
    }

    const nestedResult = toLookup
      .map(({ identifier, source }) => lookup(identifier, source, cwd))
      .filter(Boolean)

    return nestedResult[0]
  }

  const getModulePath = (sourcePath, fileName, cwd) => {
    const aliasRegexIdx = aliasesRegexes.findIndex((aliasRegex) =>
      aliasRegex.test(sourcePath)
    )
    const relativeFileName = node_path.relative(cwd, fileName)
    const aliasKey = aliasesKeys[aliasRegexIdx]
    const alias = aliases[aliasKey]?.[0]
    let modulePath = ''

    if (alias) {
      let relative = alias

      if (aliasKey.endsWith('*')) {
        const aliasKeyPrefix = aliasKey.replace('*', '')

        relative = alias.replace('*', sourcePath.replace(aliasKeyPrefix, ''))
      }

      modulePath = node_path.resolve(cwd, relative)
    } else {
      // we need ../ to skip current file name
      modulePath = node_path.resolve(relativeFileName, '../' + sourcePath)
    }

    return modulePath
  }

  return {
    visitor: {
      ImportDeclaration(path, { filename, cwd }) {
        const sourceRelative = (source) => {
          const rel = node_path.relative(node_path.dirname(filename), source)

          return rel.startsWith('.') ? rel : './' + rel
        }

        const node = path.node
        const source = node.source

        if (source.type !== 'StringLiteral') {
          return
        }

        const shouldSkip = node[SKIP] || !isPathRelativeOrAliased(source.value)

        if (shouldSkip) {
          return
        }

        const modulePath = getModulePath(source.value, filename, cwd)
        const defaultSpecifier = node.specifiers.find(
          (specifier) => specifier.type === 'ImportDefaultSpecifier'
        )

        const namespaceSpecifier = node.specifiers.find(
          (specifier) => specifier.type === 'ImportNamespaceSpecifier'
        )

        const specifiers = node.specifiers.filter(
          (specifier) => specifier.type === 'ImportSpecifier'
        )

        const results = specifiers.map((specifier) => {
          const importedName = specifier.imported.name
          const result = lookup(importedName, modulePath, cwd)

          if (!result) {
            return {
              identifier: importedName,
              local: specifier.local.name,
              source: source.value
            }
          }

          cache.set(cacheKey(importedName, modulePath), result)

          return {
            ...result,
            source: sourceRelative(result.source),
            local: specifier.local.name
          }
        })

        const defaultResult = defaultSpecifier
          ? lookup('default', modulePath, cwd)
          : null

        if (defaultResult) {
          cache.set(cacheKey('default', modulePath), defaultResult)
        }

        const buildNamed = template(`
          import { %%IMPORT_NAME%% } from %%SOURCE%%;
        `)

        const buildNamedWithAlias = template(`
          import { %%IMPORTED_NAME%% as %%LOCAL_NAME%% } from %%SOURCE%%;
        `)

        const buildDefault = template(`
          import %%IMPORT_NAME%% from %%SOURCE%%;
        `)

        const buildNamespace = template(`
          import * as %%IMPORT_NAME%% from %%SOURCE%%;
        `)

        const defaultImport = defaultResult
          ? [
              buildDefault({
                IMPORT_NAME: types.identifier(defaultSpecifier.local.name),
                SOURCE: types.stringLiteral(
                  sourceRelative(defaultResult.source)
                )
              })
            ]
          : defaultSpecifier
          ? [
              buildDefault({
                IMPORT_NAME: types.identifier(defaultSpecifier.local.name),
                SOURCE: types.stringLiteral(source.value)
              })
            ]
          : []

        const namespaceImport = namespaceSpecifier
          ? [
              buildNamespace({
                IMPORT_NAME: types.identifier(namespaceSpecifier.local.name),
                SOURCE: types.stringLiteral(source.value)
              })
            ]
          : []

        const named = results.map(({ type, identifier, local, source }) => {
          if (type === 'default') {
            return buildDefault({
              IMPORT_NAME: types.identifier(identifier),
              SOURCE: types.stringLiteral(source)
            })
          } else if (identifier !== local) {
            return buildNamedWithAlias({
              IMPORTED_NAME: types.identifier(identifier),
              LOCAL_NAME: types.identifier(local),
              SOURCE: types.stringLiteral(source)
            })
          } else {
            return buildNamed({
              IMPORT_NAME: types.identifier(identifier),
              SOURCE: types.stringLiteral(source)
            })
          }
        })

        const newImports = [...namespaceImport, ...defaultImport, ...named].map(
          (node) => {
            node[SKIP] = true

            return node
          }
        )

        path.replaceWithMultiple(newImports)
      }
    }
  }
}
