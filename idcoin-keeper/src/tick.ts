export function createSerializedTickRunner(tick: () => Promise<void>, warn: (message: string) => void): () => Promise<void> {
  let running = false
  let skippedInStreak = 0
  return async () => {
    if (running) {
      skippedInStreak++
      if (skippedInStreak === 1) warn('previous keeper tick still running; skipping overlapping tick')
      return
    }
    skippedInStreak = 0
    running = true
    try {
      await tick()
    } finally {
      running = false
    }
  }
}
