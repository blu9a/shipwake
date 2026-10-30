// Generates TypeScript types from the JSON Schema that the Go model produces (spec/shipwake.schema.json).
import { readFileSync, writeFileSync } from 'node:fs';
import { compile } from 'json-schema-to-typescript';

const schema = JSON.parse(readFileSync(new URL('../../spec/shipwake.schema.json', import.meta.url)));
const ts = await compile(schema, 'Shipwake', {
  bannerComment: '// Generated from spec/shipwake.schema.json by scripts/types.mjs. Do not edit.',
  additionalProperties: false,
  unreachableDefinitions: true,
});
writeFileSync(new URL('../src/model.gen.ts', import.meta.url), ts);
console.log('wrote src/model.gen.ts');
