#!/usr/bin/env node

const package = require('./package.json')
const { Command } = require('commander')

const { find } = require('./find')
const program = new Command('rev-dep')
program.version(package.version, '-v, --version')

const pathToString = (str, f, i) => {
  return `${str ? `${str}\n` : ''}${' '.repeat(i)} ➞ ${f}`
}

program
  .command('resolve <filePath> <entryPoints...>')
  .option(
    '-cs, --compactSummary',
    'print a compact summary of reverse resolution with a count of found paths'
  )
  .option('--verbose', 'print current action information')
  .option(
    '-wc, --webpackConfig <path>',
    'path to webpack config to enable webpack aliases support'
  )
  .action((filePath, entryPoints, data) => {
    const { compactSummary, verbose, webpackConfig } = data
    const results = find({
      entryPoints,
      filePath,
      verbose,
      webpackConfig
    })
    const hasAnyResults = results.some((paths) => paths.length > 0)
    if (!hasAnyResults) {
      console.log('No results found for', filePath, 'in', entryPoints)
      return
    }
    console.log('Results:\n')
    if (compactSummary) {
      const maxEntryLength = entryPoints.reduce((maxLength, entryPoint) => {
        return entryPoint.length > maxLength ? entryPoint.length : maxLength
      }, 0)
      let total = 0
      entryPoints.forEach((entry, index) => {
        console.log(`${entry.padEnd(maxEntryLength)} :`, results[index].length)
        total += results[index].length
      })
      console.log('\nTotal:', total)
    } else {
      results.forEach((entryPointResults, index) => {
        entryPointResults.forEach((path) => {
          console.log(path.reduce(pathToString, ''))
        })
        if (index < results.length - 1) {
          console.log('_'.repeat(process.stdout.columns))
        }
      })
    }
  })

program.parse(process.argv)
