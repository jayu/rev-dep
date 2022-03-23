type OptionMeta = [string, string]

export const tsConfigOption: OptionMeta = [
  '-tc, --tsConfig <path>',
  'path to TypeScript config to enable TS aliases support'
]

export type TsConfigOptionType = {
  tsConfig?: string
}

export const webpackConfigOption: OptionMeta = [
  '-wc, --webpackConfig <path>',
  'path to webpack config to enable webpack aliases support'
]

export type WebpackConfigOptionType = {
  webpackConfig?: string
}

export const cwdOption: OptionMeta = [
  '--cwd <path>',
  'path to a directory that should be used as a resolution root'
]

export type CwdOptionType = {
  cwd?: string
}

export const reexportRewireOption: OptionMeta = [
  '--rr reexportRewire <value>',
  'resolve actual dependencies for "export * from" statements'
]

export type ReexportRewireOptionType = {
  reexportRewire?: boolean
}
