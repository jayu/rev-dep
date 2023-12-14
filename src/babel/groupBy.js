export function groupBy(arr, property) {
  return arr.reduce((result, obj) => {
    const key = obj[property]

    if (!result[key]) {
      result[key] = []
    }

    result[key].push(obj)

    return result
  }, {})
}
