// Feature B - has circular dependency with featureA
import { featureA } from './featureA';

export function featureB() {
    console.log('Feature B');
    featureA();
}
