// Toca no grafo de áudio o PCM 16 bits estéreo que chega do concord-audio.
const RATE = 48000
const ms = (n) => Math.round((RATE * n) / 1000) * 2 // amostras intercaladas L R, sempre par
// Os blocos chegam a cada ~10 ms e o áudio é consumido em fatias de 2,67 ms:
// sem uma folga inicial, qualquer atraso vira estalo.
const PREBUFFER = ms(40)
// Os relógios da placa de som e do navegador nunca batem; se o atraso passar
// disso, volta para a folga inicial em vez de acumular para sempre.
const LIMIT = ms(120)

class PcmPlayer extends AudioWorkletProcessor {
  constructor() {
    super()
    this.buf = new Float32Array(RATE * 2)
    this.read = 0
    this.write = 0
    this.size = 0
    this.playing = false
    this.port.onmessage = (e) => {
      const pcm = new Int16Array(e.data)
      for (let i = 0; i < pcm.length; i++) {
        this.buf[this.write] = pcm[i] / 32768
        this.write = (this.write + 1) % this.buf.length
      }
      this.size = Math.min(this.size + pcm.length, this.buf.length)
      if (this.size > LIMIT) {
        const skip = this.size - PREBUFFER
        this.read = (this.read + skip) % this.buf.length
        this.size -= skip
      }
      if (this.size >= PREBUFFER) this.playing = true
    }
  }

  process(_, outputs) {
    const [left, right] = outputs[0]
    for (let i = 0; i < left.length; i++) {
      if (this.playing && this.size >= 2) {
        left[i] = this.buf[this.read]
        right[i] = this.buf[(this.read + 1) % this.buf.length]
        this.read = (this.read + 2) % this.buf.length
        this.size -= 2
      } else {
        // Esvaziou: fica em silêncio até juntar a folga de novo.
        this.playing = false
        left[i] = 0
        right[i] = 0
      }
    }
    return true
  }
}

registerProcessor('pcm-player', PcmPlayer)
