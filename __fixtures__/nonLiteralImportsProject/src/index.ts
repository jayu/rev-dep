export const loadPage = (name: string) => import('./pages/' + name);
export const loadAny = (path: string) => import(path);
export const loadReal = () => import('./real');
