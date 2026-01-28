export class Button {
  constructor(private label: string) {}
  
  render() {
    return `<button>${this.label}</button>`;
  }
}
