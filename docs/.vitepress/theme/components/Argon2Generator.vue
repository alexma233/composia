<script setup lang="ts">
import { computed, ref } from 'vue'

const password = ref('')
const hash = ref('')
const error = ref('')
const copying = ref(false)
const generating = ref(false)

const canGenerate = computed(() => password.value.length > 0 && !generating.value)

async function generateHash() {
  if (!password.value || generating.value) {
    return
  }

  generating.value = true
  error.value = ''

  try {
    const { argon2id } = await import('hash-wasm')
    const salt = crypto.getRandomValues(new Uint8Array(16))

    hash.value = await argon2id({
      password: password.value,
      salt,
      parallelism: 4,
      iterations: 3,
      memorySize: 65536,
      hashLength: 32,
      outputType: 'encoded',
    })
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Failed to generate Argon2 hash.'
  } finally {
    generating.value = false
  }
}

async function copyHash() {
  if (!hash.value || copying.value) {
    return
  }

  copying.value = true

  try {
    await navigator.clipboard.writeText(hash.value)
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Failed to copy hash.'
  } finally {
    copying.value = false
  }
}
</script>

<template>
  <div class="argon2-generator">
    <p class="argon2-generator__title">Argon2 Password Generator</p>
    <p class="argon2-generator__hint">
      Runs entirely in your browser and generates an Argon2id hash you can paste into
      <code>WEB_LOGIN_PASSWORD_HASH</code>.
    </p>

    <label class="argon2-generator__label" for="argon2-password">Password</label>
    <input
      id="argon2-password"
      v-model="password"
      class="argon2-generator__input"
      type="password"
      autocomplete="new-password"
      placeholder="Enter a password"
    />

    <div class="argon2-generator__actions">
      <button class="argon2-generator__button" type="button" :disabled="!canGenerate" @click="generateHash">
        {{ generating ? 'Generating...' : 'Generate hash' }}
      </button>
      <button class="argon2-generator__button argon2-generator__button--secondary" type="button" :disabled="!hash" @click="copyHash">
        {{ copying ? 'Copying...' : 'Copy hash' }}
      </button>
    </div>

    <label class="argon2-generator__label" for="argon2-hash">Encoded hash</label>
    <textarea
      id="argon2-hash"
      class="argon2-generator__output"
      :value="hash"
      readonly
      rows="4"
      placeholder="Generated $argon2id hash appears here"
    />

    <p v-if="error" class="argon2-generator__error">{{ error }}</p>
  </div>
</template>

<style scoped>
.argon2-generator {
  margin: 1rem 0;
  padding: 1rem;
  border: 1px solid var(--vp-c-divider);
  border-radius: 14px;
  background: var(--vp-c-bg-soft);
}

.argon2-generator__title {
  margin: 0;
  font-weight: 600;
}

.argon2-generator__hint {
  margin: 0.5rem 0 1rem;
  color: var(--vp-c-text-2);
  font-size: 0.95rem;
}

.argon2-generator__label {
  display: block;
  margin: 0 0 0.4rem;
  font-size: 0.9rem;
  font-weight: 600;
}

.argon2-generator__input,
.argon2-generator__output {
  box-sizing: border-box;
  width: 100%;
  padding: 0.8rem 0.9rem;
  border: 1px solid var(--vp-c-divider);
  border-radius: 10px;
  background: var(--vp-c-bg);
  color: var(--vp-c-text-1);
  font: inherit;
}

.argon2-generator__output {
  resize: vertical;
}

.argon2-generator__actions {
  display: flex;
  gap: 0.75rem;
  margin: 0.9rem 0 1rem;
  flex-wrap: wrap;
}

.argon2-generator__button {
  border: 0;
  border-radius: 999px;
  padding: 0.7rem 1rem;
  background: var(--vp-c-brand-1);
  color: var(--vp-c-neutral-inverse);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}

.argon2-generator__button--secondary {
  background: var(--vp-c-default-2);
  color: var(--vp-c-text-1);
}

.argon2-generator__button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.argon2-generator__error {
  margin: 0.8rem 0 0;
  color: var(--vp-c-danger-1);
}
</style>
