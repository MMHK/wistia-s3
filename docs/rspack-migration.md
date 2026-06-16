# Rspack Migration

## Goal
Migrate `web/` from webpack 5 to rspack for faster builds.

## Technical Approach
- Replace webpack with `@rspack/core` + `@rspack/cli`
- Use built-in SWC instead of babel-loader
- Use `experiments.css = true` (CSS bundled into JS, injected at runtime)
- Custom inline plugin to inline JS into HTML (replaces html-inline-script-webpack-plugin + HTMLInlineCSSWebpackPlugin)
- `excludeChunks` on `rspack.HtmlRspackPlugin` replaces html-webpack-exclude-assets-plugin
- `@rspack/dev-server` replaces webpack-dev-server
- `CopyRspackPlugin` for favicon.ico

## Affected files
- `web/rspack.config.js` (new)
- `web/webpack.config.js` (keep for reference, not deleted)
- `web/package.json` (deps + scripts)

## Tasks
- [x] Create `web/rspack.config.js`
- [x] Update `web/package.json` (remove webpack deps, add rspack deps, update scripts)
- [x] Add FRP dev server tunnel support (mmhk-frp + inquirer)
- [x] Install dependencies
- [x] Run `yarn build` and verify output
- [x] Update AGENTS.md if needed
