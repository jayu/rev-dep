export type CodeModification = {
  modificationCode: string
}

export type CodeChange = CodeModification

export type CodeChangeWithLocation = CodeModification & MatchPosition

export type MatchPosition = {
  start: number
  end: number
  loc: Location
}

type ModifyCodeAsTextParams = {
  code: string
  modificationCode?: string
  alreadyChangedCodes?: string[]
  location: Pick<MatchPosition, 'start' | 'end'>
}

type ProcessCodeModificationsArrayParams = {
  code: string
  changes: CodeChangeWithLocation[]
}

export const regExpTest = (regExp: RegExp, text: string) => {
  if (!text) {
    return false
  }

  const matches = text.match(regExp)

  return matches !== null && matches.length > 0
}

export function modifyCodeAsText({
  code,
  modificationCode,
  location
}: ModifyCodeAsTextParams) {
  let fileCode = code

  const codeBeforeMatch = fileCode.slice(0, location.start as number)
  const codeAfterMatch = fileCode.slice(location.end as number)
  const replacedCodeLength = location.end - location.start
  const replacementCodeLength = (modificationCode as string).length
  const locationsChange = {
    from: location.end,
    to: location.end + replacementCodeLength - replacedCodeLength
  }

  fileCode = `${codeBeforeMatch}${modificationCode}${codeAfterMatch}`

  return { fileCode, locationsChange }
}

export function processTextCodeModificationsArray({
  code,
  changes
}: ProcessCodeModificationsArrayParams) {
  let modifiedCode = code

  /**
   * Include only changes that are unique by it's location.
   * Remove changes that are inside range of other changes
   */
  const pendingChanges = changes.filter(
    (change, changeIdx) =>
      !changes.some(
        (otherChange, otherChangeIdx) =>
          otherChangeIdx !== changeIdx &&
          otherChange.start <= change.start &&
          otherChange.end >= change.end &&
          // insert changes has the same start and end to distinguish them from anchor node, that might have other changes attached
          change.start !== change.end
      )
  )

  while (pendingChanges.length > 0) {
    const change = pendingChanges.shift() as CodeChangeWithLocation

    const { locationsChange, fileCode } = modifyCodeAsText({
      code: modifiedCode,
      modificationCode: change.modificationCode,
      location: { start: change.start, end: change.end }
    })

    modifiedCode = fileCode

    pendingChanges.forEach((pendingChange) => {
      if (pendingChange.start >= locationsChange.from) {
        const diff = locationsChange.to - locationsChange.from
        pendingChange.end += diff
        pendingChange.start += diff
      }
    })
  }

  return modifiedCode
}
