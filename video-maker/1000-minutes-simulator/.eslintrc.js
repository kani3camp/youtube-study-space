module.exports = {
    env: {
        es2021: true,
    },
    extends: [
        'eslint:recommended',
        'plugin:@typescript-eslint/recommended',
        'next/core-web-vitals',
        'google',
        'prettier',
    ],
    plugins: ['unused-imports'],
    rules: {
        'require-jsdoc': ['off'], // 必要に応じて変更してください。
        'import/order': ['error', { alphabetize: { order: 'asc' } }],
        '@next/next/no-img-element': ['off'],
        '@typescript-eslint/no-unused-vars': 'off',
        'unused-imports/no-unused-imports-ts': 'warn',
    },
}
