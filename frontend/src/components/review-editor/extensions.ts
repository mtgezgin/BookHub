import { type Extensions, Mark, mergeAttributes } from "@tiptap/core";
import { Blockquote } from "@tiptap/extension-blockquote";
import { Bold } from "@tiptap/extension-bold";
import { Document } from "@tiptap/extension-document";
import { Italic } from "@tiptap/extension-italic";
// Alt yoldan içe aktarma: paketin kökü görev listesi gibi kullanmadığımız parçaları da getirir (adım 1: 8 kB).
import { BulletList } from "@tiptap/extension-list/bullet-list";
import { ListItem } from "@tiptap/extension-list/item";
import { OrderedList } from "@tiptap/extension-list/ordered-list";
import { Paragraph } from "@tiptap/extension-paragraph";
import { Text } from "@tiptap/extension-text";
import { Placeholder } from "@tiptap/extensions/placeholder";
import { UndoRedo } from "@tiptap/extensions/undo-redo";

/*
 * İnceleme düzenleyicisinin ŞEMASI (Faz 17C). Sunucudaki izin listesinin (backend/internal/richtext)
 * aynısı: paragraf, alıntı, iki liste, kalın, italik, spoiler. Şemada olmayan şey belgeye giremez;
 * yapıştırılan başlık, bağlantı ya da resim düz metne iner.
 *
 * Bu yalnız kullanım kolaylığıdır. İsteği herkes elle gönderebilir; asıl kapı sunucudaki doğrulayıcı.
 */

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    spoiler: { toggleSpoiler: () => ReturnType };
  }
}

/**
 * Spoiler bir İŞARETTİR (kalın gibi): aynı metnin üstünde kalın ve italikle birlikte durabilir.
 * Klavye kısayolu bilerek yok: Ctrl+Shift+S Firefox ve Edge'de ekran görüntüsü aracıdır (spec adım 1).
 */
const Spoiler = Mark.create({
  name: "spoiler",
  parseHTML: () => [{ tag: "span[data-spoiler]" }],
  renderHTML: ({ HTMLAttributes }) => ["span", mergeAttributes(HTMLAttributes, { "data-spoiler": "" }), 0],
  addCommands() {
    return { toggleSpoiler: () => ({ commands }) => commands.toggleMark(this.name) };
  },
});

/**
 * Özniteliksiz sıralı liste. Varsayılanı JSON'a `attrs: {start, type}` yazar; sunucu hiçbir düğümde
 * attrs kabul etmez.
 */
const PlainOrderedList = OrderedList.extend({
  addAttributes() {
    return {};
  },
});

/**
 * Tab ve Shift+Tab bağlanmamış liste maddesi. Varsayılanı Tab'ı girintiye bağlar; o zaman klavyeyle
 * gezen okur listenin içinden çıkmak için içeriği değiştirmek zorunda kalır. Burada Tab her zaman
 * odağı ilerletir (klavye tuzağı yok, #128 kriter 7). Bedeli: düzenleyicide iç içe liste yazılamaz;
 * yapıştırılan iç içe liste korunur ve okuma tarafı onu çizer.
 */
const FlatListItem = ListItem.extend({
  addKeyboardShortcuts() {
    return { Enter: () => this.editor.commands.splitListItem(this.name) };
  },
});

export function reviewExtensions(placeholder: string): Extensions {
  return [
    Document, Paragraph, Text,
    Bold, Italic, Spoiler,
    Blockquote, BulletList, PlainOrderedList, FlatListItem,
    UndoRedo,
    Placeholder.configure({ placeholder }),
  ];
}
