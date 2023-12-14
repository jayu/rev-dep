export function template(template: string) {
  return (params: Record<string, string>) => {
    let code = template.trim()
    Object.entries(params).forEach(([key, value]) => {
      code = code.replace(new RegExp(`%%${key}%%`, 'g'), value)
    })
    return code
  }
}
