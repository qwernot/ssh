import { useCallback, useRef, useState } from 'react'

/**
 * useZmodem - ZModem/lrzsz file transfer detection and handling for xterm.js
 * Detects rz/sz commands in terminal output and handles file transfer
 */
export function useZmodem() {
  const [transferring, setTransferring] = useState(false)
  const [transferName, setTransferName] = useState('')
  const [transferProgress, setTransferProgress] = useState(0)
  const zmodemDetected = useRef(false)

  // ZModem header/footer detection patterns
  const ZMODEM_HEADER = new Uint8Array([0x18, 0x42, 0x30, 0x30]) // **B00
  const ZMODEM_HEX = '**\x18B'

  const detectZmodem = useCallback((data: Uint8Array | string): boolean => {
    if (data instanceof Uint8Array) {
      // Check for ZModem header bytes
      for (let i = 0; i < data.length - 3; i++) {
        if (data[i] === 0x18 && data[i + 1] === 0x42 && data[i + 2] === 0x30 && data[i + 3] === 0x30) {
          return true
        }
      }
    } else if (typeof data === 'string') {
      return data.includes(ZMODEM_HEX)
    }
    return false
  }, [])

  const startReceive = useCallback((filename: string) => {
    setTransferring(true)
    setTransferName(filename)
    setTransferProgress(0)
    zmodemDetected.current = true
  }, [])

  const updateProgress = useCallback((percent: number) => {
    setTransferProgress(percent)
  }, [])

  const complete = useCallback(() => {
    setTransferring(false)
    setTransferName('')
    setTransferProgress(0)
    zmodemDetected.current = false
  }, [])

  const cancel = useCallback(() => {
    setTransferring(false)
    setTransferName('')
    setTransferProgress(0)
    zmodemDetected.current = false
  }, [])

  return {
    transferring,
    transferName,
    transferProgress,
    detectZmodem,
    startReceive,
    updateProgress,
    complete,
    cancel,
    isDetected: zmodemDetected.current,
  }
}
