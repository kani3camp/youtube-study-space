import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { isDeepStrictEqual } from 'node:util';

const read = (path) => JSON.parse(readFileSync(new URL(path, import.meta.url), 'utf8'));
const normalize = (index) => ({
  collectionGroup: index.collectionGroup,
  queryScope: index.queryScope,
  fields: index.fields.map((field) => ({ ...field, fieldPath: field.fieldPath.replaceAll('`', '') })),
});

test('deploys the composite index required by the MyPage snapshot query', () => {
  // The emulator does not enforce composite indexes; Go query tests cannot catch removal.
  const required = normalize(read('../docs/mypage/query-index.json'));
  assert.ok(read('./firestore.indexes.json').indexes.some((index) =>
    isDeepStrictEqual(normalize(index), required)));
});
