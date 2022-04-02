module.exports = {
  "hooks": {
    "before:init": ["npm run checks"],
    // Docs are generated using build CLI, so we have to build first
    "after:bump": ["npm run build", "npm run docs-gen"]
  },
  "git": {
    "commitMessage": "chore: release v${version}",
    "commit": true,
    "tag": true,
    "push": true,
  }
}