/**
 * Structured logging with secret redaction. Every log line passes through
 * redactSecrets() so WIFs / mnemonic word sequences can never leak via logs.
 */
const WIF_RE = /\b[5KL][1-9A-HJ-NP-Za-km-z]{50,51}\b|\bc[1-9A-HJ-NP-Za-km-z]{50,51}\b/g
const HEXKEY_RE = /\b[0-9a-fA-F]{64}\b/g

const MNEMONIC_WORDS = new Set<string>()

export function registerMnemonicForRedaction(mnemonic: string): void {
  for (const w of mnemonic.trim().split(/\s+/)) {
    if (w.length >= 3) MNEMONIC_WORDS.add(w)
  }
}

export function redactSecrets(input: string): string {
  let out = input.replace(WIF_RE, '[REDACTED-WIF]')
  out = out.replace(HEXKEY_RE, (m) => '[REDACTED-HEX]')
  if (MNEMONIC_WORDS.size > 0) {
    out = out.replace(/\b[a-z]+\b/g, (w) => (MNEMONIC_WORDS.has(w) ? '[REDACTED-WORD]' : w))
  }
  return out
}

export type LogLevel = 'info' | 'warn' | 'error' | 'alert'

export interface LogRecord {
  ts: string
  level: LogLevel
  msg: string
  [k: string]: any
}

export class Logger {
  private sink: (line: string) => void
  readonly lines: string[] = []

  constructor(sink?: (line: string) => void) {
    this.sink = sink || ((line) => console.log(line))
  }

  private emit(level: LogLevel, msg: string, fields?: Record<string, any>) {
    const rec: LogRecord = { ts: new Date().toISOString(), level, msg, ...(fields || {}) }
    const line = redactSecrets(JSON.stringify(rec))
    this.lines.push(line)
    this.sink(line)
  }

  info(msg: string, fields?: Record<string, any>) { this.emit('info', msg, fields) }
  warn(msg: string, fields?: Record<string, any>) { this.emit('warn', msg, fields) }
  error(msg: string, fields?: Record<string, any>) { this.emit('error', msg, fields) }
  /** loud operational alert — key compromise, fee anomaly, etc. */
  alert(msg: string, fields?: Record<string, any>) { this.emit('alert', msg, fields) }
}
