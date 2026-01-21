// This file has a missing import
import { nonExistentPkgFunction } from 'non-existent-pkg';

export function testFunction() {
    nonExistentPkgFunction();
}
