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
 * + support imports from baseUrl from TS config -> relative | baseUrl | alias
 * +  persist the original import alias
 * + allow for a list of files to rewire
 * + use cache for not resolved modules as well
 * + handle type imports properly - we don't preserve the import was a type import
 * + do not touch imports that don't need changes
 */

module.exports = function plugin(
  { types },
  {
    tsConfigPath = findTsConfig(),
    cache = new Map(),
    includeBarrelExportFiles,
    excludeBarrelExportFiles = []
  }
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
  const makeRegExpFromAliasExpression = (aliasExpression) => {
    return new RegExp(`^${aliasExpression.replace('*', '(.+)')}$`)
  }

  const aliasesRegexes = Object.keys(aliases).map(makeRegExpFromAliasExpression)

  // TODO we assume that only one aliased path can exist
  const aliasedPathRegExps = Object.values(aliases).map(([fistAliasedPath]) =>
    makeRegExpFromAliasExpression(fistAliasedPath)
  )

  const interpolateAliasWithPath = (
    aliasKey,
    aliasedPathRegExp,
    resolvedSourcePathRelativeToBaseUrl
  ) => {
    const [_, ...groups] = aliasedPathRegExp.exec(
      resolvedSourcePathRelativeToBaseUrl
    )

    const aliasParts = aliasKey.split('*')
    const interpolatedAlias = aliasParts.reduce(
      (mergedPath, aliasPart, idx) => {
        return `${mergedPath}${aliasPart}${groups[idx] ?? ''}`
      },
      ''
    )

    return interpolatedAlias
  }

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

    const path = node_path.normalize(paths[0])
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

    const isRelative = path.startsWith('.')
    const isAbsolute = path.startsWith('/')

    const isBaseUrlPath = baseUrlDirs.some((dir) => path.startsWith(dir))

    return aliasRegexIdx > -1 || isRelative || isAbsolute || isBaseUrlPath
  }

  const getCacheKey = (identifier, filePath) => `${identifier}-${filePath}`

  const lookup = (identifier, filePath, cwd) => {
    const cached = cache.get(getCacheKey(identifier, filePath))

    if (cached) {
      return { ...cached, isCached: true }
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
      return { resolvedAs: null, visitedFiles: [] }
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
                      // Here we could check if identifier comes from import statement, and if so, lookup deeper
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
        return { resolvedAs, visitedFiles: [resolvedAs.source] }
      }

      const nestedResult = toLookup
        .map(({ identifier, source }) => lookup(identifier, source, cwd))
        .filter((lookUpResult) => lookUpResult.resolvedAs !== null)

      if (nestedResult[0]) {
        return {
          resolvedAs: nestedResult[0].resolvedAs,
          visitedFiles: [resolvedFilePath, ...nestedResult[0].visitedFiles]
        }
      }

      return { resolvedAs: null, visitedFiles: [] }
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
    const alias = aliases[aliasKey]?.[0] // TODO we assume that only one aliased path can exist in config

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

    return node_path.normalize(modulePath)
  }

  const getImportKind = (sourcePath) => {
    const aliasRegexIdx = aliasesRegexes.findIndex((aliasRegex) =>
      aliasRegex.test(sourcePath)
    )

    const isRelative = sourcePath.startsWith('.')

    const isBaseUrlPath = baseUrlDirs.some((dir) => sourcePath.startsWith(dir))

    if (aliasRegexIdx > -1) {
      return 'aliased'
    }
    if (isRelative) {
      return 'relative'
    }
    if (isBaseUrlPath) {
      return 'baseUrl'
    }

    throw new Error('Could not determine import kind')
  }

  return {
    visitor: {
      Program() {
        // console.log('Cache size', cache.size)
      },
      ImportDeclaration(path, state) {
        const filename = state.filename

        const getImportSourceFormatted = (resolvedSourcePath, importKind) => {
          const baseDirPath = node_path.join(root, baseUrl)

          if (importKind === 'baseUrl') {
            const relativeToBaseUrl = node_path.relative(
              baseDirPath,
              resolvedSourcePath
            )

            return relativeToBaseUrl
          }
          if (importKind === 'aliased') {
            const originalSource = path.node.source.value
            const currentAliasIdx = aliasesRegexes.findIndex((aliasRegex) =>
              aliasRegex.test(originalSource)
            )

            const resolvedSourcePathRelativeToBaseUrl = resolvedSourcePath
              .replace(baseDirPath, '')
              .replace(/^\//, '')

            // Try to use current alias if it matches new path
            if (currentAliasIdx > -1) {
              const aliasKey = aliasesKeys[currentAliasIdx]
              const aliasedPathRegExp = aliasedPathRegExps[currentAliasIdx]

              if (aliasedPathRegExp.test(resolvedSourcePathRelativeToBaseUrl)) {
                return interpolateAliasWithPath(
                  aliasKey,
                  aliasedPathRegExp,
                  resolvedSourcePathRelativeToBaseUrl
                )
              }
            }

            // Try finding matching alias
            const newMatchingAliasIndex = aliasedPathRegExps.findIndex(
              (aliasedPathRegexp) =>
                aliasedPathRegexp.test(resolvedSourcePathRelativeToBaseUrl)
            )

            if (newMatchingAliasIndex > -1) {
              const aliasKey = aliasesKeys[newMatchingAliasIndex]
              const aliasedPathRegExp =
                aliasedPathRegExps[newMatchingAliasIndex]

              return interpolateAliasWithPath(
                aliasKey,
                aliasedPathRegExp,
                resolvedSourcePathRelativeToBaseUrl
              )
            }
          }

          const rel = node_path.relative(
            node_path.dirname(filename),
            resolvedSourcePath
          )

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

        if (node.specifiers.length === 0) {
          // Skip imports without 'from'
          return
        }

        const shouldSkip = node[SKIP] || !shouldPathBeAnalyzed(source.value)

        if (shouldSkip) {
          return
        }

        const importKind = getImportKind(source.value)
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

          if (!result?.isCached) {
            const cacheKey = getCacheKey(importedName, modulePath)

            // console.log('resolved not cached', cacheKey, result)

            const originalImport = {
              identifier: importedName,
              local: specifier.local.name,
              source: source.value // cannot cache non absolute path
            }

            const originalImportToCache = {
              identifier: importedName,
              local: specifier.local.name,
              source: modulePath
            }

            const originalResolution = {
              resolvedAs: originalImportToCache,
              visitedFiles: []
            }

            if (!result.resolvedAs) {
              cache.set(cacheKey, originalResolution)

              return originalImport
            }

            if (
              includeBarrelExportFiles &&
              !includeBarrelExportFiles.some((fileThatHasToBeVisited) =>
                result.visitedFiles.includes(fileThatHasToBeVisited)
              )
            ) {
              cache.set(cacheKey, originalResolution)
              return originalImport
            }

            if (
              excludeBarrelExportFiles.some((fileThatCannotBeVisited) =>
                result.visitedFiles.includes(fileThatCannotBeVisited)
              )
            ) {
              cache.set(cacheKey, originalResolution)
              return originalImport
            }

            cache.set(cacheKey, result)
          }

          return {
            ...result.resolvedAs,
            source: getImportSourceFormatted(
              result.resolvedAs.source,
              importKind
            ),
            local: specifier.local.name
          }
        })

        const defaultResult = defaultSpecifier
          ? lookup('default', modulePath, root)
          : null

        if (defaultResult && !defaultResult.isCached) {
          const cacheKey = getCacheKey('default', modulePath)

          const originalImportToCache = {
            source: modulePath
          }

          const originalResolution = {
            resolvedAs: originalImportToCache,
            visitedFiles: []
          }

          if (!defaultResult.resolvedAs) {
            cache.set(cacheKey, originalResolution)
          } else if (
            includeBarrelExportFiles &&
            !includeBarrelExportFiles.some((fileThatHasToBeVisited) =>
              defaultResult.visitedFiles.includes(fileThatHasToBeVisited)
            )
          ) {
            cache.set(cacheKey, originalResolution)
          } else if (
            excludeBarrelExportFiles.some((fileThatCannotBeVisited) =>
              defaultResult.visitedFiles.includes(fileThatCannotBeVisited)
            )
          ) {
            cache.set(cacheKey, originalResolution)
          } else {
            cache.set(cacheKey, defaultResult)
          }
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

        const defaultImport = defaultResult?.resolvedAs
          ? [
              buildDefault({
                IMPORT_NAME: defaultSpecifier.local.name,
                SOURCE: getImportSourceFormatted(
                  defaultResult.resolvedAs.source,
                  importKind
                )
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
                      ? `${identifier} as ${local}`
                      : identifier
                  )
                  .join(', ')
              : null

          return `import ${isTypeImport ? 'type ' : ''}${
            defaultPart ? `${defaultPart}${nonDefaultPart ? ', ' : ''}` : ''
          }${nonDefaultPart ? `{ ${nonDefaultPart} }` : ''} from '${source}';`
        })

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
