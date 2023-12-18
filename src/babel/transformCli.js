const rootPath = process.argv[2]
const inputFilePath = process.argv[3]

import { transform } from './transform'

if (!rootPath) {
  console.error('Please provide correct transformation root')
  process.exit(1)
}

const run = async () => {
  const startTime = new Date().getTime()
  await transform({
    rootPath,
    inputFilePath
  })

  console.log('Operation time: ', (new Date().getTime() - startTime) / 1000)
}

run()
