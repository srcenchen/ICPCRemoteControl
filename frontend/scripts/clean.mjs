// Clean old files from web/ but preserve the broadcast/ subdirectory
import { rmSync, existsSync, readdirSync, statSync } from 'fs'
import { join, resolve } from 'path'
import { fileURLToPath } from 'url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))
const webDir = resolve(__dirname, '../../internal/server/web')

if (!existsSync(webDir)) process.exit(0)

for (const entry of readdirSync(webDir)) {
  if (entry === 'broadcast') continue // keep broadcast display pages
  const fullPath = join(webDir, entry)
  rmSync(fullPath, { recursive: true, force: true })
  console.log('Removed:', fullPath)
}

console.log('Clean complete.')
