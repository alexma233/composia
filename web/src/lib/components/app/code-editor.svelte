<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';

  import { defaultKeymap, indentWithTab } from '@codemirror/commands';
  import { markdown } from '@codemirror/lang-markdown';
  import { yaml } from '@codemirror/lang-yaml';
  import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language';
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

  const languageCompartment = new Compartment();
  const editableCompartment = new Compartment();

  onMount(() => {
    editorView = new EditorView({
      state: EditorState.create({
        doc: value,
        extensions: [
          basicSetup,
          lineNumbers(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
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
          EditorView.theme({
            '&': { height: '100%', backgroundColor: 'transparent' },
            '.cm-scroller': { overflow: 'auto', fontFamily: 'var(--font-mono, monospace)' },
            '.cm-content': { minHeight: '100%' },
            '.cm-gutters': { backgroundColor: 'transparent', borderRight: '1px solid hsl(var(--border))' }
          })
        ]
      }),
      parent: host
    });
  });

  onDestroy(() => editorView?.destroy());

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

<div bind:this={host} class="h-full min-h-0"></div>
