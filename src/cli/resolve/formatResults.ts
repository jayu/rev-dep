import { InputParams } from './types'

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
    return
  }
  formatted += join('Results:\n')
  if (compactSummary) {
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
      entryPointResults.forEach((path) => {
        formatted += join(path.reduce(pathToString, ''), '\n')
      })
      if (index < results.length - 1 && entryPointResults.length > 0) {
        formatted += join('_'.repeat(process.stdout.columns))
      }
    })
  }
  return formatted
}
