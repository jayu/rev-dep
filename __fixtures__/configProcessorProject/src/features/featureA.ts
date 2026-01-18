// Feature A - has circular dependency with featureB
import { featureB } from './featureB';

export function featureA() {
    console.log('Feature A');
    featureB();
}
