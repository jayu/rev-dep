import leftPad from 'left-pad';
import { helper } from './helper';

export function padName(name: string): string {
  return helper(leftPad(name, 10));
}
