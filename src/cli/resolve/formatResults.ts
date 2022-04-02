import { InputParams } from './types'
import * as colors from 'colorette'

type Results = Array<Array<Array<string>>>

const pathToString = (str: string, filePath: string, indentation: number) => {
  return `${str ? `${str}\n` : ''}${' '.repeat(indentation)} ➞ ${filePath}`
}

const join = (...args: any[]) => args.join(' ') + '\n'

export function formatResults({
  results,
  filePath,
  entryPoints,
  compactSummary
}: {
  results: Results
  compactSummary: InputParams['compactSummary']
  entryPoints: string[]
  filePath: string
}) {
  let formatted = ''
  const hasAnyResults = results.some((paths) => paths.length > 0)
  if (!hasAnyResults) {
    formatted = join('No results found for', filePath, 'in', entryPoints)
    return formatted
  }

  if (compactSummary) {
    formatted += join('Results:\n')
    const maxEntryLength = entryPoints.reduce((maxLength, entryPoint) => {
      return entryPoint.length > maxLength ? entryPoint.length : maxLength
    }, 0)
    let total = 0
    entryPoints.forEach((entry, index) => {
      formatted += join(
        `${entry.padEnd(maxEntryLength)} :`,
        results[index].length
      )
      total += results[index].length
    })
    formatted += join('\nTotal:', total)
  } else {
    results.forEach((entryPointResults, index) => {
      if (entryPointResults.length > 0) {
        formatted += join(colors.bold(entryPoints[index]), ':', '\n')
        entryPointResults.forEach((path, resultsIndex) => {
          const isLast = resultsIndex === entryPointResults.length - 1
          formatted += join(path.reduce(pathToString, ''), isLast ? '' : '\n')
        })
        if (index < results.length - 1 && entryPointResults.length > 0) {
          formatted += join('_'.repeat(process.stdout.columns)) + '\n'
        }
      }
    })
  }
  return formatted
}
