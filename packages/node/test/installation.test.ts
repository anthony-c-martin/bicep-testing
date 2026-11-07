import { Bicep } from '@azure/bicep-rpc-client';
import { access, mkdtemp, readFile, readdir, rm, writeFile } from 'fs/promises';
import os from 'os';
import path from 'path';
import { BicepTestSession } from '../src';

let home: string;
const version = '0.47.16';
const executableName = process.platform === 'win32' ? 'bicep.exe' : 'bicep';
const initializationError = new Error('Expected initialization sentinel');
const cachePath = () => path.join(home, '.bicep', 'bin', `v${version}`);

beforeEach(async () => {
  home = await mkdtemp(path.join(os.tmpdir(), 'bicep-install-test-'));
  jest.spyOn(os, 'homedir').mockReturnValue(home);
  jest.spyOn(Bicep, 'initialize').mockRejectedValue(initializationError);
});

afterEach(async () => {
  jest.restoreAllMocks();
  await rm(home, { recursive: true, force: true });
});

test('reuses an executable already installed in the cache', async () => {
  const install = jest.spyOn(Bicep, 'install').mockImplementation(async directory => {
    const executable = path.join(directory, executableName);
    await writeFile(executable, 'cached-cli', { mode: 0o755 });
    return executable;
  });

  await expect(BicepTestSession.create(version)).rejects.toBe(initializationError);
  await expect(BicepTestSession.create(version)).rejects.toBe(initializationError);

  expect(install).toHaveBeenCalledTimes(1);
  expect(await readFile(path.join(cachePath(), executableName), 'utf8')).toBe('cached-cli');
  expect(await readdir(cachePath())).toEqual([executableName]);
  expect(Bicep.initialize).toHaveBeenNthCalledWith(1, path.join(cachePath(), executableName));
  expect(Bicep.initialize).toHaveBeenNthCalledWith(2, path.join(cachePath(), executableName));
});

test('concurrent uncached installs publish one complete executable without overwriting it', async () => {
  let releaseDownloads: () => void = () => { throw new Error('Download barrier is not initialized'); };
  const downloadsReady = new Promise<void>(resolve => { releaseDownloads = resolve; });
  let downloads = 0;
  const install = jest.spyOn(Bicep, 'install').mockImplementation(async directory => {
    const executable = path.join(directory, executableName);
    const download = ++downloads;
    await writeFile(executable, `download-${download}`, { mode: 0o755 });
    if (download === 2) {
      releaseDownloads();
    }
    await downloadsReady;
    return executable;
  });

  await Promise.all([
    expect(BicepTestSession.create(version)).rejects.toBe(initializationError),
    expect(BicepTestSession.create(version)).rejects.toBe(initializationError),
  ]);

  expect(install).toHaveBeenCalledTimes(2);
  expect(await readFile(path.join(cachePath(), executableName), 'utf8')).toMatch(/^download-[12]$/);
  expect(await readdir(cachePath())).toEqual([executableName]);
  expect(Bicep.initialize).toHaveBeenCalledTimes(2);
});

test('failed downloads propagate their error and remove temporary install directories', async () => {
  const downloadError = new Error('Download failed');
  jest.spyOn(Bicep, 'install').mockRejectedValue(downloadError);

  await expect(BicepTestSession.create(version)).rejects.toBe(downloadError);

  expect(Bicep.initialize).not.toHaveBeenCalled();
  expect(await readdir(cachePath())).toEqual([]);
  await expect(access(path.join(cachePath(), executableName))).rejects.toMatchObject({ code: 'ENOENT' });
});
