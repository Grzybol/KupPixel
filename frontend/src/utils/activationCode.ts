const activationCodeRegex = /^[A-Za-z0-9]{4}(?:-[A-Za-z0-9]{4}){3}$/;

export function normalizeActivationCode(input: string): string {
  return input.trim().toUpperCase();
}

export function isActivationCodeValid(input: string): boolean {
  return activationCodeRegex.test(normalizeActivationCode(input));
}

export function getActivationCodePattern(): RegExp {
  return activationCodeRegex;
}
