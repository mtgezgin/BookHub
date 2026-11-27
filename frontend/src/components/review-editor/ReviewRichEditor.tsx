import type { Editor } from "@tiptap/core";
import { EditorContent, useEditor, useEditorState } from "@tiptap/react";
import { type KeyboardEvent, useId, useRef, useState } from "react";
import type { DocNode } from "../../lib/reviewDoc";
import { cn } from "../../lib/utils";
import "./editor.css";
import { reviewExtensions } from "./extensions";

/*
 * Zengin metin düzenleyicisi (Faz 17C). AYRI PARÇA: yalnız "İnceleme yaz" ya da "Düzenle" denince
 * yüklenir (MyReview.tsx'teki lazy); inceleme okuyan ziyaretçiye inmez. Tiptap ve altındaki ProseMirror
 * gzip ~100 kB.
 *
 * Belgeyi JSON olarak verir (onChange); HTML hiçbir yere gitmez. Uzunluk, boşluk ve sınır denetimi
 * çağıranın işi (lib/reviewDoc).
 */

interface Props {
  /** Açılıştaki belge. Sonradan değişse de editör yeniden kurulmaz: yazılan metin korunur (409 akışı). */
  initial: DocNode;
  onChange: (doc: DocNode) => void;
  /** Editörü adlandıran görünür etiketin kimliği. */
  labelledBy: string;
  invalid: boolean;
}

/** Araç çubuğu düğmeleri. Metinleri görünür: simge ezberletmez, erişilebilir ad ile görünen ad aynı. */
const TOOLS = [
  { key: "bold", label: "Kalın", hint: "Ctrl+B", className: "font-bold", run: (e: Editor) => e.chain().focus().toggleBold().run() },
  { key: "italic", label: "İtalik", hint: "Ctrl+I", className: "italic", run: (e: Editor) => e.chain().focus().toggleItalic().run() },
  { key: "spoiler", label: "Spoiler", hint: "Seçili metni okuyandan gizler", className: "", run: (e: Editor) => e.chain().focus().toggleSpoiler().run() },
  { key: "blockquote", label: "Alıntı", hint: "Ctrl+Shift+B", className: "", run: (e: Editor) => e.chain().focus().toggleBlockquote().run() },
  { key: "bulletList", label: "Madde listesi", hint: "Ctrl+Shift+8", className: "", run: (e: Editor) => e.chain().focus().toggleBulletList().run() },
  { key: "orderedList", label: "Numaralı liste", hint: "Ctrl+Shift+7", className: "", run: (e: Editor) => e.chain().focus().toggleOrderedList().run() },
] as const;

const FIELD =
  "w-full min-w-0 rounded-md border border-input bg-card px-3 py-2 text-base shadow-xs transition-[color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/30 aria-invalid:border-destructive md:text-sm";

export default function ReviewRichEditor({ initial, onChange, labelledBy, invalid }: Readonly<Props>) {
  const editorId = useId();
  // Uzantılar bir kez kurulur: her çizimde yeni dizi, kancaya "seçenekler değişti" dedirtirdi.
  const [extensions] = useState(() => reviewExtensions("Kitap hakkında ne düşünüyorsun?"));
  // aria-invalid de burada: useEditor her çizimde seçenekleri karşılaştırır ve değişeni editöre uygular.
  // (Özniteliği bir etkiden `editor.view.dom`'a yazmak derlenmiş pakette çöküyordu: görünüm o anda henüz
  // bağlanmamış oluyor; jsdom'daki birim testi bunu göstermedi, Playwright gösterdi.)
  const editor = useEditor({
    extensions,
    content: initial,
    injectCSS: false,
    editorProps: {
      attributes: {
        id: editorId, role: "textbox", "aria-multiline": "true", "aria-labelledby": labelledBy,
        "aria-invalid": String(invalid), class: FIELD,
      },
    },
    onUpdate: ({ editor: e }) => onChange(e.getJSON() as DocNode),
  });

  if (!editor) return null;
  return (
    <div className="review-editor flex flex-col gap-2">
      <Toolbar editor={editor} controls={editorId} />
      <EditorContent editor={editor} />
    </div>
  );
}

/**
 * Biçim araç çubuğu (WAI-ARIA toolbar kalıbı): Tab çubuğa TEK durakla girer, ok tuşları düğmeler arasında
 * gezer (Home ve End uçlara), Tab çubuktan çıkar. Basılı durum aria-pressed ile söylenir.
 *
 * Fareyle tıklamada mousedown engellenir: düğme odağı almaz, editördeki seçim kaybolmaz. Klavyeyle
 * basıldığında komut odağı editöre geri taşır; okur yazmaya devam eder.
 */
function Toolbar({ editor, controls }: Readonly<{ editor: Editor; controls: string }>) {
  const active = useEditorState({
    editor,
    selector: ({ editor: e }) => Object.fromEntries(TOOLS.map((t) => [t.key, e.isActive(t.key)])),
  });
  const [current, setCurrent] = useState(0);
  const buttons = useRef<(HTMLButtonElement | null)[]>([]);

  const onKeyDown = (e: KeyboardEvent) => {
    const last = TOOLS.length - 1;
    const next = { ArrowRight: current === last ? 0 : current + 1, ArrowLeft: current === 0 ? last : current - 1, Home: 0, End: last }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    setCurrent(next);
    buttons.current[next]?.focus();
  };

  return (
    <div role="toolbar" aria-label="Biçim" aria-controls={controls} className="flex flex-wrap gap-1" onKeyDown={onKeyDown}>
      {TOOLS.map((tool, i) => (
        <button
          key={tool.key}
          ref={(el) => {
            buttons.current[i] = el;
          }}
          type="button"
          tabIndex={i === current ? 0 : -1}
          aria-pressed={active[tool.key]}
          title={tool.hint}
          onMouseDown={(e) => e.preventDefault()}
          onFocus={() => setCurrent(i)}
          onClick={() => tool.run(editor)}
          className={cn(
            "inline-flex h-8 items-center rounded-md border border-input bg-card px-2.5 text-sm shadow-xs transition-colors outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground",
            tool.className,
          )}
        >
          {tool.label}
        </button>
      ))}
    </div>
  );
}
