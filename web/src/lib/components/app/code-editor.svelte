<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';

  import { defaultKeymap, indentWithTab } from '@codemirror/commands';
  import { githubDark } from '@fsegurai/codemirror-theme-github-dark';
  import { githubLight } from '@fsegurai/codemirror-theme-github-light';
  import { markdown } from '@codemirror/lang-markdown';
  import { yaml } from '@codemirror/lang-yaml';
  import { EditorState, Compartment } from '@codemirror/state';
  import { EditorView, keymap, lineNumbers } from '@codemirror/view';
  import { basicSetup } from 'codemirror';

  const dispatch = createEventDispatcher<{
    change: { value: string };
    save: void;
  }>();

  export let value = '';
  export let path = '';
  export let readOnly = false;

  let host: HTMLDivElement;
  let editorView: EditorView | null = null;
  let rootObserver: MutationObserver | null = null;

  const languageCompartment = new Compartment();
  const editableCompartment = new Compartment();
  const themeCompartment = new Compartment();

  const editorChromeTheme = EditorView.theme({
    '&': {
      height: '100%',
      borderRadius: '0.75rem',
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
          keymap.of([
            indentWithTab,
            ...defaultKeymap,
            {
              key: 'Mod-s',
              run: () => {
                dispatch('save');
                return true;
              }
            }
          ]),
          languageCompartment.of(languageExtension(path)),
          editableCompartment.of(EditorView.editable.of(!readOnly)),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              dispatch('change', { value: update.state.doc.toString() });
            }
          }),
        ]
      }),
      parent: host
    });

    rootObserver = new MutationObserver(() => syncTheme(root));
    rootObserver.observe(root, {
      attributes: true,
      attributeFilter: ['class', 'data-theme-mode']
    });

    return () => {
      rootObserver?.disconnect();
      rootObserver = null;
    };
  });

  onDestroy(() => {
    rootObserver?.disconnect();
    rootObserver = null;
    editorView?.destroy();
  });

  $: if (editorView) {
    const currentValue = editorView.state.doc.toString();
    if (currentValue !== value) {
      editorView.dispatch({
        changes: { from: 0, to: currentValue.length, insert: value }
      });
    }

    editorView.dispatch({
      effects: [
        languageCompartment.reconfigure(languageExtension(path)),
        editableCompartment.reconfigure(EditorView.editable.of(!readOnly))
      ]
    });
  }

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

  function languageExtension(filePath: string) {
    if (filePath.endsWith('.yaml') || filePath.endsWith('.yml')) {
      return yaml();
    }
    if (filePath.endsWith('.md')) {
      return markdown();
    }
    return [];
  }
</script>

<div bind:this={host} class="h-full min-h-0 overflow-hidden rounded-xl"></div>
