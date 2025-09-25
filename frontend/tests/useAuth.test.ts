// @ts-ignore - node type definitions are not available in this environment
import assert from "node:assert/strict";
import { parseUser } from "../src/useAuth.js";

declare const process: { exitCode?: number };

function test(name: string, fn: () => void) {
  try {
    fn();
    console.log(`✓ ${name}`);
  } catch (error) {
    console.error(`✗ ${name}`);
    console.error(error);
    process.exitCode = 1;
  }
}

test("parseUser returns null when nested user is null", () => {
  const result = parseUser({ user: null });
  assert.equal(result, null);
});
