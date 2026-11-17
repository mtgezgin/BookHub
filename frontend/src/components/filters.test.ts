import { describe, expect, it } from "vitest";
import { autoSearchable } from "./filters";

// Yazarken arama (Enter'sız) hangi metinde yapılır: kuralın gerekçesi filters.tsx'te.
describe("autoSearchable", () => {
  it.each([
    ["", true], // kutu boşaldı: arama kalkar
    ["s", false],
    ["sa", false],
    ["sav", true],
    ["savaş", true],
    ["çğ", false], // iki harf (kod noktası), dört bayt
    ["çğş", true],
    ["us", false],
    ["ince m", false], // yeni kelimenin ilk harfi
    ["ince me", true],
    ["a b", false],
    ["a bc", true], // çok kelimede ölçüt son kelimedir
    ["savaş ve b", false],
    ["savaş ve ba", true],
    ["ince  me", true], // art arda boşluk tek ayraç sayılır
  ])("%j → %s", (text, want) => {
    expect(autoSearchable(text)).toBe(want);
  });
});
