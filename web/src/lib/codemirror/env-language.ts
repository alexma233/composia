import { StreamLanguage } from "@codemirror/language";

type EnvLanguageState = {
  inValue: boolean;
  quote: '"' | "'" | null;
};

export const envLanguage = StreamLanguage.define<EnvLanguageState>({
  name: "dotenv",
  startState() {
    return { inValue: false, quote: null };
  },
  token(stream, state) {
    if (stream.sol()) {
      state.inValue = false;
      state.quote = null;
    }

    if (stream.eatSpace()) {
      return null;
    }

    if (!state.inValue) {
      if (stream.peek() === "#") {
        stream.skipToEnd();
        return "comment";
      }

      if (stream.match(/[A-Za-z_][A-Za-z0-9_.-]*/)) {
        return "variableName";
      }

      if (stream.match(/[=:]/)) {
        state.inValue = true;
        return "operator";
      }

      stream.next();
      return "invalid";
    }

    if (state.quote) {
      while (!stream.eol()) {
        const character = stream.next();
        if (
          character === state.quote &&
          (state.quote !== '"' || stream.string[stream.pos - 2] !== "\\")
        ) {
          state.quote = null;
          break;
        }
      }
      return "string";
    }

    const next = stream.peek();
    if (next === '"' || next === "'") {
      state.quote = next;
      stream.next();
      return "string";
    }

    if (next === "#" && isInlineCommentStart(stream)) {
      stream.skipToEnd();
      return "comment";
    }

    stream.next();
    while (!stream.eol()) {
      if (stream.peek() === "#" && isInlineCommentStart(stream)) {
        break;
      }
      stream.next();
    }
    return "string";
  },
});

function isInlineCommentStart(stream: {
  pos: number;
  string: string;
}): boolean {
  return stream.pos === 0 || /\s/.test(stream.string[stream.pos - 1] ?? "");
}
