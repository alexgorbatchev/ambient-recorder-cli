import { expect, test } from "bun:test";
import { mkdir, mkdtemp, realpath, stat, symlink } from "node:fs/promises";
import { join } from "node:path";
import { devBootstrap } from "../devBootstrap";

async function setup() {
  await mkdir(".tmp", { recursive: true });
  const rootPath = await realpath(await mkdtemp(".tmp/dev-bootstrap-test-"));
  const dotfilesPath = join(rootPath, "dotfiles with spaces");
  const binariesPath = join(dotfilesPath, ".generated/binaries");
  const payloadPath = join(binariesPath, "ambient-recorder/v1/ambient-recorder");
  const shimPath = join(dotfilesPath, ".generated/bin/ambient-recorder");
  const sourcePath = join(rootPath, "source");
  await mkdir(join(binariesPath, "ambient-recorder/v1"), { recursive: true });
  await mkdir(join(dotfilesPath, ".generated/bin"), { recursive: true });
  await symlink("v1", join(binariesPath, "ambient-recorder/current"));
  await Bun.write(payloadPath, "old payload");
  await Bun.write(sourcePath, "new payload");
  await Bun.write(shimPath,
    `TOOL_EXECUTABLE="${binariesPath}/ambient-recorder/current/ambient-recorder"\n`);
  return { dotfilesPath, payloadPath, shimPath, sourcePath, rootPath };
}

test("replaces the shim-selected payload and preserves the current symlink", async () => {
  const paths = await setup();
  expect(await devBootstrap(paths.sourcePath, paths.dotfilesPath)).toBe(paths.payloadPath);
  expect(await Bun.file(paths.payloadPath).text()).toBe("new payload");
  expect((await stat(paths.payloadPath)).mode & 0o777).toBe(0o755);
  expect(await realpath(join(paths.dotfilesPath, ".generated/binaries/ambient-recorder/current")))
    .toBe(join(paths.dotfilesPath, ".generated/binaries/ambient-recorder/v1"));
});

test.each([
  "TOOL_EXECUTABLE=relative\n",
  'TOOL_EXECUTABLE="$HOME/payload"\n',
  'TOOL_EXECUTABLE="/one"\nTOOL_EXECUTABLE="/two"\n',
])("rejects unsafe shim assignment %s", async (shimText) => {
  const paths = await setup();
  await Bun.write(paths.shimPath, shimText);
  await expect(devBootstrap(paths.sourcePath, paths.dotfilesPath)).rejects.toThrow(
    "Ambient recorder shim must contain one literal, absolute TOOL_EXECUTABLE path.",
  );
  expect(await Bun.file(paths.payloadPath).text()).toBe("old payload");
});

test("rejects a payload outside the dotfiles binaries directory", async () => {
  const paths = await setup();
  await Bun.write(paths.shimPath, `TOOL_EXECUTABLE="${paths.sourcePath}"\n`);
  await expect(devBootstrap(paths.sourcePath, paths.dotfilesPath)).rejects.toThrow(
    "Ambient recorder payload must be inside the dotfiles binaries directory.",
  );
  expect(await Bun.file(paths.sourcePath).text()).toBe("new payload");
});

test("rejects an empty build without replacing the installed payload", async () => {
  const paths = await setup();
  await Bun.write(paths.sourcePath, "");
  await expect(devBootstrap(paths.sourcePath, paths.dotfilesPath)).rejects.toThrow("Build output is empty.");
  expect(await Bun.file(paths.payloadPath).text()).toBe("old payload");
});
