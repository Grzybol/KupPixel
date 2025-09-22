// @ts-ignore - node type definitions are not available in this environment
import assert from "node:assert/strict";
import { getActivationCodePattern, isActivationCodeValid, normalizeActivationCode } from "../src/utils/activationCode.js";
import en from "../src/lang/en.json" with { type: "json" };
import pl from "../src/lang/pl.json" with { type: "json" };

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

test("normalizeActivationCode trims and uppercases input", () => {
  const input = "  abcd-efgh-ijkl-mnop  ";
  const expected = "ABCD-EFGH-IJKL-MNOP";
  assert.equal(normalizeActivationCode(input), expected);
});

test("isActivationCodeValid accepts properly formatted codes", () => {
  const validCodes = [
    "ABCD-EFGH-IJKL-MNOP",
    "1234-5678-9ABC-DEF0",
    "zzzz-zzzz-zzzz-zzzz",
  ];
  for (const code of validCodes) {
    assert.equal(isActivationCodeValid(code), true, `${code} should be valid`);
  }
});

test("isActivationCodeValid rejects invalid codes", () => {
  const invalidCodes = [
    "abcd-efgh-ijkl", // too short
    "abcd-efgh-ijkl-mnop-qrst", // too long
    "abcd_efgh_ijkl_mnop", // wrong separator
    "abc!-efgh-ijkl-mnop", // invalid character
    "", // empty
  ];
  for (const code of invalidCodes) {
    assert.equal(isActivationCodeValid(code), false, `${code} should be invalid`);
  }
});

test("normalizeActivationCode output matches API pattern", () => {
  const normalized = normalizeActivationCode("a1b2-c3d4-e5f6-g7h8");
  assert.equal(getActivationCodePattern().test(normalized), true);
});

function collectKeys(record: Record<string, unknown>): string[] {
  return Object.keys(record).sort();
}

test("password reset translations are aligned", () => {
  const enPasswordReset = (en.auth as Record<string, unknown>).passwordReset as Record<string, unknown>;
  const plPasswordReset = (pl.auth as Record<string, unknown>).passwordReset as Record<string, unknown>;
  assert.ok(enPasswordReset, "english password reset section missing");
  assert.ok(plPasswordReset, "polish password reset section missing");
  assert.deepEqual(collectKeys(enPasswordReset), collectKeys(plPasswordReset));
  const enErrors = enPasswordReset.errors as Record<string, unknown>;
  const plErrors = plPasswordReset.errors as Record<string, unknown>;
  assert.deepEqual(collectKeys(enErrors), collectKeys(plErrors));
});
