# Website

This website is built using [Docusaurus](https://docusaurus.io/), a modern static website generator.

## Installation

```bash
pnpm install
```

## Local Development

```bash
pnpm start
```

This command starts a local development server and opens up a browser window. Most changes are reflected live without having to restart the server.

## Build

```bash
pnpm build
```

This command generates static content into the `build` directory and can be served using any static contents hosting service.

## GitHub Pages Deployment

This repo is set up to deploy the docs from the local terminal to the `gh-pages` branch, matching the Docusaurus GitHub Pages flow.

### Repository settings

In GitHub, configure:

1. `Settings -> Pages`
2. `Source: Deploy from a branch`
3. `Branch: gh-pages`
4. `Folder: / (root)`

### Deploy using SSH

The repository remote already uses SSH (`git@github.com:jayu/rev-dep.git`), so the simplest deploy command is:

```bash
pnpm deploy:gh-pages
```

This script:

1. Builds the static site.
2. Runs `docusaurus deploy`.
3. Pushes the generated output to the `gh-pages` branch.

### Preview the production build locally

```bash
pnpm build:pages
pnpm serve:build
```

### Alternate target: default GitHub Pages URL

If you want to publish to the default project pages URL instead of the custom domain, deploy with:

```bash
DOCS_URL=https://jayu.github.io DOCS_BASE_URL=/rev-dep/ pnpm deploy:gh-pages
```

### Custom domain

The current setup defaults to `https://rev-dep.com/`. The `static/CNAME` file is included so GitHub Pages keeps that custom domain on deploy.

### HTTPS deploy without SSH

If you ever need HTTPS instead of SSH, use:

```bash
GIT_USER=<your-github-username> pnpm deploy
```

GitHub will ask for a personal access token instead of a password.
