import chalk from 'chalk';
import missing from 'genuinely-undeclared-module';

export function color(text: string): string {
  return chalk.green(missing(text));
}
