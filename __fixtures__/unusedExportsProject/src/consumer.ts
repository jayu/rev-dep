import { helper } from './utils';
import type { Foo } from './types';

const x: Foo = { name: 'test' };
console.log(helper(), x);
