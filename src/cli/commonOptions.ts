type OptionMeta2 = [string, string]
type OptionMeta3 = [string, string, string]

export const webpackConfigOption: OptionMeta2 = [
  '-wc, --webpackConfig <path>',
  'path to webpack config to enable webpack aliases support'
]

export type WebpackConfigOptionType = {
  webpackConfig?: string
}

export const cwdOption: OptionMeta3 = [
  '--cwd <path>',
  'path to a directory that should be used as a resolution root',
  process.cwd()
]

export type CwdOptionType = {
  cwd: string
}

export const reexportRewireOption: OptionMeta2 = [
  '--rr reexportRewire <value>',
  'resolve actual dependencies for "export * from" statements'
]

export type ReexportRewireOptionType = {
  reexportRewire?: boolean
}
