/**
 * Write chore, docs, refactor, test to CHANGELOG.md
 * @see https://d.potato4d.me/entry/20200920-standard-version-customize/
 */
module.exports = {
  preset: require.resolve("conventional-changelog-conventionalcommits"),
  types: [
    { type: "feat", section: "Features" },
    { type: "fix", section: "Bug Fixes" },
    { type: "build", section: "Build Changes", hidden: false },
    { type: "ci", section: "CI Changes", hidden: false },
    { type: "chore", section: "Chores", hidden: false },
    { type: "docs", section: "Document Changes", hidden: false },
    { type: "refactor", section: "Refactoring", hidden: false },
    { type: "test", section: "Test Improvements", hidden: false },
  ],
};
