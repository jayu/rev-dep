// In public boundary
import { privateFunc } from './private';

export function publicFunc() {
    console.log('Public function');
    privateFunc(); // This should be a boundary violation
}
