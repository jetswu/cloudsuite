// Password generation for admin-created accounts.
// Uses crypto.getRandomValues (CSPRNG) so passwords are generated client-side
// and never travel through a deterministic PRNG.

const UPPER = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
const LOWER = "abcdefghijklmnopqrstuvwxyz"
const DIGIT = "0123456789"
const SYMBOL = "!@#$%^&*()-_=+[]{}"

// randomInt returns a uniformly distributed integer in [0, max) using
// rejection sampling to avoid modulo bias.
function randomInt(max: number): number {
  const range = 256 - (256 % max)
  const buf = new Uint8Array(1)
  let value = 0
  do {
    crypto.getRandomValues(buf)
    value = buf[0]
  } while (value >= range)
  return value % max
}

function pick(alphabet: string): string {
  return alphabet[randomInt(alphabet.length)]
}

// shuffle mutates a copy of the slice with a Fisher–Yates shuffle using
// crypto.getRandomValues, so the ordering of the final password is not
// predictable from the character-class layout.
function shuffle(chars: string[]): string[] {
  const out = chars.slice()
  for (let i = out.length - 1; i > 0; i--) {
    const j = randomInt(i + 1)
    ;[out[i], out[j]] = [out[j], out[i]]
  }
  return out
}

// generatePassword returns a random password with at least one uppercase
// letter, one lowercase letter, one digit, and one symbol. The remaining
// characters are drawn uniformly from all four classes.
export function generatePassword(length = 16): string {
  if (length < 4) {
    throw new Error("password length must be at least 4")
  }
  const chars: string[] = [
    pick(UPPER),
    pick(LOWER),
    pick(DIGIT),
    pick(SYMBOL),
  ]
  const all = UPPER + LOWER + DIGIT + SYMBOL
  for (let i = chars.length; i < length; i++) {
    chars.push(pick(all))
  }
  return shuffle(chars).join("")
}
