const fs = require('fs')
const path = require("path")

const newVersion = process.argv[2]

if (!newVersion) {
  console.log('Provide new version number as a parameter')
  process.exit(1)
}

const mainPkgJsonPath = './npm/rev-dep/package.json'
const mainPkgJson = JSON.parse(fs.readFileSync(mainPkgJsonPath).toString())

const nativePackages = []

console.log(mainPkgJson.optionalDependencies);

mainPkgJson.optionalDependencies = Object.fromEntries(Object.entries(mainPkgJson.optionalDependencies).map(([pkg]) => {
  nativePackages.push(pkg)
  return [pkg, newVersion]
}))

const longestPackageName = Math.max(...nativePackages.map((name) => name.length))

console.log('Updated', 'rev-dep'.padEnd(longestPackageName, ' '), 'version from', mainPkgJson.version, "to", newVersion)

mainPkgJson.version = newVersion

fs.writeFileSync(mainPkgJsonPath, JSON.stringify(mainPkgJson, null, 2))

for (const nativePkgName of nativePackages) {
  const pathToPackage = path.join("npm", nativePkgName, "package.json")
  const pkgJson = JSON.parse(fs.readFileSync(pathToPackage).toString())
  console.log('Updated', nativePkgName.padEnd(longestPackageName, ' '), "version from", pkgJson.version, "to", newVersion)
  pkgJson.version = newVersion
  fs.writeFileSync(pathToPackage, JSON.stringify(pkgJson, null, 2))
}

const versionFile = `package main

var Version = "${newVersion}"`

fs.writeFileSync("./version.go", versionFile)

console.log("Saved version.go file")