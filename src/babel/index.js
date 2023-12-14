/*eslint-disable @typescript-eslint/no-var-requires */

const node_path = require('path')
const fs = require('fs')
const parser = require('@babel/parser')
import { findTsConfig } from '../lib/utils'
import { template } from './template'
const SKIP = Symbol('SKIP')
import { babelParsingOptions } from './babelParsingOptions'
import { groupBy } from './groupBy'
/**
 *
 * TODO
 * + If that has to be used as a codemod, we have to refactor to make sure we don't change structure of other parts of the code and we preserve imports order
 * +- group named imports from the same file
 * - support imports from baseUrl from TS config
 * - persist the original import alias
 * - allow for a list of files to rewire
 * + handle type imports properly - we don't preserve the import was a type import
 * - do not touch imports that don't need changes
 */

module.exports = function plugin(
  { types },
  { tsConfigPath = findTsConfig(), cache = new Map() }
) {
  const root = tsConfigPath.replace('/tsconfig.json', '')
  const tsConfigContent = fs.readFileSync(tsConfigPath).toString()
  const tsConfigContentCleaned = tsConfigContent
    // remove comments
    .replace(/^(\s)*\/\//gm, '')
    .replace(/\/\*.+?\*\//gm, '')

  const tsConfig = JSON.parse(tsConfigContentCleaned)
  const aliases = tsConfig.compilerOptions.paths
  const aliasesKeys = Object.keys(aliases)
  const aliasesRegexes = Object.keys(aliases).map((alias) => {
    return new RegExp(`^${alias.replace('*', '(.)+')}$`)
  })

  let baseUrlDirs = []

  const baseUrl = tsConfig.compilerOptions.baseUrl

  if (baseUrl) {
    const baseDirPath = node_path.join(root, baseUrl)

    const dirNames = fs
      .readdirSync(baseDirPath, { withFileTypes: true })
      .filter((dirent) => dirent.isDirectory())
      .map((dirent) => dirent.name + '/')

    baseUrlDirs = dirNames
  }

  const getFile = (original, paths) => {
    if (paths.length === 0) {
      console.warn('Cannot resolve import ' + original)
      return null
    }

    const path = paths[0]
    try {
      return [path, fs.readFileSync(path).toString()]
    } catch (e) {
      return getFile(original, paths.slice(1))
    }
  }

  const shouldPathBeAnalyzed = (path) => {
    const aliasRegexIdx = aliasesRegexes.findIndex((aliasRegex) =>
      aliasRegex.test(path)
    )

    const isRelative = path.startsWith('./') || path.startsWith('../')
    const isAbsolute = path.startsWith('/')

    const isBaseUrlPath = baseUrlDirs.some((dir) => path.startsWith(dir))

    return aliasRegexIdx > -1 || isRelative || isAbsolute || isBaseUrlPath
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

    const fileInfo = getFile(filePath, withExtension)

    if (!fileInfo) {
      return null
    }

    const [resolvedFilePath, file] = fileInfo

    try {
      const ast = parser.parse(file, babelParsingOptions)

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
            if (
              declaration.declaration?.type.startsWith('TS') &&
              declaration.declaration?.type.endsWith('Declaration')
            ) {
              const typeName = declaration.declaration.id.name
              if (typeName === identifier) {
                resolvedAs = {
                  // This should be 'type' of something else, but ESLint would handle that
                  type: 'named',
                  identifier,
                  source: filePath
                }
              }
            } else if (types.isVariableDeclaration(declaration.declaration)) {
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
                    } else if (shouldPathBeAnalyzed(source)) {
                      toLookup.push({
                        identifier: specifier.local.name,
                        source: getModulePath(source, resolvedFilePath, cwd)
                      })
                    }
                  }
                }
              })
            }
          } else if (
            types.isExportAllDeclaration(declaration) &&
            shouldPathBeAnalyzed(declaration.source.value)
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
    } catch (e) {
      console.log('Lookup parse error', filePath, e)
      process.exit(0)
    }
  }

  const getModulePath = (sourcePath, fileName, cwd) => {
    const aliasRegexIdx = aliasesRegexes.findIndex((aliasRegex) =>
      aliasRegex.test(sourcePath)
    )
    const relativeFileName = node_path.relative(cwd, fileName)
    const aliasKey = aliasesKeys[aliasRegexIdx]
    const alias = aliases[aliasKey]?.[0]

    const isAbsoluteToBaseDir = baseUrlDirs.some((baseUrlDir) =>
      sourcePath.startsWith(baseUrlDir)
    )

    let modulePath = ''

    if (alias) {
      let relative = alias

      if (aliasKey.endsWith('*')) {
        const aliasKeyPrefix = aliasKey.replace('*', '')

        relative = alias.replace('*', sourcePath.replace(aliasKeyPrefix, ''))
      }

      modulePath = node_path.resolve(cwd, relative)
    } else if (isAbsoluteToBaseDir) {
      modulePath = node_path.join(cwd, sourcePath)
    } else {
      // we need ../ to skip current file name
      modulePath = node_path.join(cwd, relativeFileName, '../' + sourcePath)
    }
    return modulePath
  }

  return {
    visitor: {
      Program() {
        // console.log('Cache size', cache.size)
      },
      ImportDeclaration(path, state) {
        const filename = state.filename

        const sourceRelative = (source) => {
          const rel = node_path.relative(node_path.dirname(filename), source)
          const whatever = rel.startsWith('.') ? rel : './' + rel
          // remove file extension
          return whatever.replace(/\.(ts|js|tsx|jsx|cjs|mjs)$/, '')
        }

        const node = path.node
        const isTypeImport = node.importKind === 'type'
        const source = node.source

        if (source.type !== 'StringLiteral') {
          return
        }

        const shouldSkip = node[SKIP] || !shouldPathBeAnalyzed(source.value)

        if (shouldSkip) {
          return
        }

        const modulePath = getModulePath(source.value, filename, root)

        const defaultSpecifier = node.specifiers.find(
          (specifier) => specifier.type === 'ImportDefaultSpecifier' // import $$ from '$$'
        )

        const namespaceSpecifier = node.specifiers.find(
          (specifier) => specifier.type === 'ImportNamespaceSpecifier' // import * as $$ from '$$'
        )

        const specifiers = node.specifiers.filter(
          (specifier) => specifier.type === 'ImportSpecifier' // import { $$ } from '$$'
        )

        const results = specifiers.map((specifier) => {
          const importedName = specifier.imported.name
          const result = lookup(importedName, modulePath, root)

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
          ? lookup('default', modulePath, root)
          : null

        if (defaultResult) {
          cache.set(cacheKey('default', modulePath), defaultResult)
        }

        const buildNamed = template(`
          import { %%IMPORT_NAME%% } from '%%SOURCE%%';
        `)

        const buildNamedWithAlias = template(`
          import { %%IMPORTED_NAME%% as %%LOCAL_NAME%% } from '%%SOURCE%%';
        `)

        const buildDefault = template(`
          import %%IMPORT_NAME%% from '%%SOURCE%%';
        `)

        const buildNamespace = template(`
          import * as %%IMPORT_NAME%% from '%%SOURCE%%';
        `)

        const defaultImport = defaultResult
          ? [
              buildDefault({
                IMPORT_NAME: defaultSpecifier.local.name,
                SOURCE: sourceRelative(defaultResult.source)
              })
            ]
          : defaultSpecifier
          ? [
              buildDefault({
                IMPORT_NAME: defaultSpecifier.local.name,
                SOURCE: source.value
              })
            ]
          : []

        const namespaceImport = namespaceSpecifier
          ? [
              buildNamespace({
                IMPORT_NAME: namespaceSpecifier.local.name,
                SOURCE: source.value
              })
            ]
          : []

        const importsFromNamedGroupedBySource = Object.values(
          groupBy(results, 'source')
        )

        console.log(
          'importsFromNamedGroupedBySource',
          importsFromNamedGroupedBySource
        )

        const named = importsFromNamedGroupedBySource.map((imports) => {
          const source = imports[0].source
          const defaultImport = imports.find(({ type }) => type === 'default')
          const nonDefault = imports.filter(({ type }) => type !== 'default')

          const defaultPart = defaultImport
            ? `${defaultImport.identifier}`
            : null

          const nonDefaultPart =
            nonDefault.length > 0
              ? nonDefault
                  .map(({ identifier, local }) =>
                    identifier !== local
                      ? `${local} as ${identifier}`
                      : identifier
                  )
                  .join(', ')
              : null

          return `import ${isTypeImport ? 'type ' : ''}${
            defaultPart ? `${defaultPart}${nonDefaultPart ? ', ' : ''}` : ''
          }${nonDefaultPart ? `{ ${nonDefaultPart} }` : ''} from '${source}'`
        })
        // const named = results.map(({ type, identifier, local, source }) => {
        //   if (type === 'default') {
        //     return buildDefault({
        //       IMPORT_NAME: identifier,
        //       SOURCE: source
        //     })
        //   } else if (identifier !== local) {
        //     return buildNamedWithAlias({
        //       IMPORTED_NAME: identifier,
        //       LOCAL_NAME: local,
        //       SOURCE: source
        //     })
        //   } else {
        //     return buildNamed({
        //       IMPORT_NAME: identifier,
        //       SOURCE: source
        //     })
        //   }
        // })

        console.log(named)

        const newImports = [...namespaceImport, ...defaultImport, ...named].map(
          (node) => {
            return node
          }
        )

        if (!state.file.metadata) {
          state.file.metadata = {}
        }

        if (!state.file.metadata[filename]) {
          state.file.metadata[filename] = []
        }

        const modification = {
          modificationCode: newImports.join('\n'),
          start: path.node.start,
          end: path.node.end,
          loc: path.node.loc
        }

        state.file.metadata[filename].push(modification)
      }
    }
  }
}
