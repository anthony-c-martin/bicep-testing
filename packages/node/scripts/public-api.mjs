import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const generatedPath = path.resolve('dist/index.d.ts');
const baselinePath = path.resolve('../../api/node/bicep-testing.d.ts');

const normalize = value => `${value
  .replace(/\r\n/g, '\n')
  .replace(/^\/\/# sourceMappingURL=.*$/gm, '')
  .trim()}\n`;

const generated = normalize(await readFile(generatedPath, 'utf8'));

if (process.argv.includes('--update')) {
  await mkdir(path.dirname(baselinePath), { recursive: true });
  await writeFile(baselinePath, generated, 'utf8');
  console.log(`Updated ${path.relative(process.cwd(), baselinePath)}`);
} else if (process.argv.includes('--check')) {
  const baseline = normalize(await readFile(baselinePath, 'utf8'));
  if (generated !== baseline) {
    console.error('Node public API has changed. Review the declarations and run npm run api:update.');
    process.exitCode = 1;
  } else {
    console.log('Node public API is up to date.');
  }
} else {
  console.error('Specify --update or --check.');
  process.exitCode = 2;
}