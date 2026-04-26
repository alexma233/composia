<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  import { defaultKeymap, indentWithTab } from '@codemirror/commands';
  import { LanguageDescription } from '@codemirror/language';
  import { languages } from '@codemirror/language-data';
  import { lintGutter, linter } from '@codemirror/lint';
  import { githubDark } from '@fsegurai/codemirror-theme-github-dark';
  import { githubLight } from '@fsegurai/codemirror-theme-github-light';
  import { EditorState, Compartment, type Extension } from '@codemirror/state';
  import { EditorView, keymap, lineNumbers } from '@codemirror/view';
  import { basicSetup } from 'codemirror';

  import { composeLinter, isComposeFilePath } from '$lib/codemirror/compose-lint';
  import { envLinter, isEnvFilePath } from '$lib/codemirror/env-lint';
  import { observeThemeChange } from '$lib/theme-observer';

  interface Props {
    value?: string;
    path?: string;
    relatedFiles?: Record<string, string>;
    readOnly?: boolean;
    onchange?: (event: { value: string }) => void;
    onsave?: () => void;
  }

  let {
    value = $bindable(''),
    path = '',
    relatedFiles = {},
    readOnly = false,
    onchange,
    onsave,
  }: Props = $props();

  let host: HTMLDivElement;
  let editorView: EditorView | null = null;
  let disconnectThemeObserver: (() => void) | null = null;
  let languageLoadRequest = 0;

  const languageCompartment = new Compartment();
  const editableCompartment = new Compartment();
  const lintCompartment = new Compartment();
  const themeCompartment = new Compartment();

  function editorBorderRadius(): string {
    return getComputedStyle(document.documentElement).getPropertyValue('--radius-xl') || '0.75rem';
  }

  const editorChromeTheme = EditorView.theme({
    '&': {
      height: '100%',
      borderRadius: editorBorderRadius(),
      overflow: 'hidden',
    },
    '&.cm-focused': {
      outline: 'none',
    },
    '.cm-scroller': {
      overflow: 'auto',
    },
    '.cm-content': {
      minHeight: '100%',
    },
  });

  // CodeMirror may resolve duplicate @codemirror/view versions in the lockfile,
  // which makes otherwise-compatible key bindings fail TypeScript checks.
  const editorKeymap = [
    indentWithTab,
    ...defaultKeymap,
    {
      key: 'Mod-s',
      run: () => {
        onsave?.();
        return true;
      }
    }
  ] as unknown as Parameters<typeof keymap.of>[0];

  onMount(() => {
    const root = document.documentElement;

    editorView = new EditorView({
      state: EditorState.create({
        doc: value,
        extensions: [
          basicSetup,
          lineNumbers(),
          themeCompartment.of(resolveTheme(root)),
          editorChromeTheme,
          keymap.of(editorKeymap),
          languageCompartment.of([]),
          lintCompartment.of(lintExtension(path)),
          editableCompartment.of(EditorView.editable.of(!readOnly)),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              onchange?.({ value: update.state.doc.toString() });
            }
          }),
        ]
      }),
      parent: host
    });

    void syncLanguage(path);

    disconnectThemeObserver = observeThemeChange(() => syncTheme(root));

    return () => {
      disconnectThemeObserver?.();
      disconnectThemeObserver = null;
    };
  });

  onDestroy(() => {
    languageLoadRequest += 1;
    disconnectThemeObserver?.();
    disconnectThemeObserver = null;
    editorView?.destroy();
    editorView = null;
  });

  $effect(() => {
    if (editorView) {
      const currentValue = editorView.state.doc.toString();
      if (currentValue !== value) {
        editorView.dispatch({
          changes: { from: 0, to: currentValue.length, insert: value }
        });
      }
    }
  });

  $effect(() => {
    if (editorView) {
      void syncLanguage(path);
      editorView.dispatch({
        effects: [
          lintCompartment.reconfigure(lintExtension(path)),
          editableCompartment.reconfigure(EditorView.editable.of(!readOnly))
        ]
      });
    }
  });

  function resolveTheme(root: HTMLElement) {
    return root.classList.contains('dark') ? githubDark : githubLight;
  }

  function syncTheme(root: HTMLElement) {
    if (!editorView) {
      return;
    }

    editorView.dispatch({
      effects: themeCompartment.reconfigure(resolveTheme(root))
    });
  }

  function syncLanguage(filePath: string) {
    if (!editorView) {
      return;
    }

    const requestId = ++languageLoadRequest;
    const description = languageDescriptionForPath(filePath);

    if (!description) {
      applyLanguageExtension(requestId, []);
      return;
    }

    if (description.support) {
      applyLanguageExtension(requestId, description.support.extension);
      return;
    }

    void description
      .load()
      .then((support) => {
        applyLanguageExtension(requestId, support.extension);
      })
      .catch((error) => {
        if (import.meta.env.DEV) {
          console.error(`Failed to load CodeMirror language support for ${filePath}.`, error);
        }
        applyLanguageExtension(requestId, []);
      });
  }

  function applyLanguageExtension(requestId: number, extension: Extension) {
    if (!editorView || requestId !== languageLoadRequest) {
      return;
    }

    editorView.dispatch({
      effects: languageCompartment.reconfigure(extension)
    });
  }

  function languageDescriptionForPath(filePath: string) {
    const fileName = filePath.split('/').pop() ?? filePath;
    if (!fileName) {
      return null;
    }

    return LanguageDescription.matchFilename(languages, fileName);
  }

  function lintExtension(filePath: string) {
    if (isComposeFilePath(filePath)) {
      return [lintGutter(), linter(composeLinter(filePath, relatedFiles))];
    }

    if (isEnvFilePath(filePath)) {
      return [lintGutter(), linter(envLinter())];
    }

    return [];
  }
</script>

<div bind:this={host} class="h-full min-h-0 overflow-hidden rounded-xl" role="textbox" aria-label="Code editor"></div>
