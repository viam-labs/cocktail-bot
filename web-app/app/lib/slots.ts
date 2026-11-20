import { titleCase } from "./display";

// A slot id is "<place>-<letter>-<shelf>-<row>": "slot-a-top-front" on the rack,
// "fridge-a-top-front" in the fridge. Ids without a shelf, like "slot-e-front",
// put the row where the shelf goes.
export const RACK = "slot";

export interface SlotId {
  place: string;
  letter: string;
  shelf: string;
  depth: string;
}

export function parseSlotId(id: string): SlotId {
  const [place = "", letter = "", shelf = "", ...depth] = id.split("-");
  return { place, letter: letter.toUpperCase(), shelf, depth: depth.join("-") };
}

// placeName is "" on the rack, else the place to show, e.g. "Fridge".
export function placeName(place: string): string {
  return place === RACK ? "" : titleCase(place);
}

const SHELF_ORDER = ["top", "bottom"];

export interface ShelfGroup<T> {
  place: string;
  shelf: string;
  rows: Map<string, T[]>;
}

// groupByShelf groups slots by place and shelf, and by row within each shelf:
// the rack's shelves top to bottom, then every other place's.
export function groupByShelf<T extends { id: string }>(slots: T[]): ShelfGroup<T>[] {
  const groups = new Map<string, ShelfGroup<T>>();
  for (const slot of slots) {
    const { place, shelf, depth } = parseSlotId(slot.id);
    const key = `${place}/${shelf}`;
    const group = groups.get(key) ?? { place, shelf, rows: new Map<string, T[]>() };
    group.rows.set(depth, [...(group.rows.get(depth) ?? []), slot]);
    groups.set(key, group);
  }
  const shelfRank = (shelf: string) => (SHELF_ORDER.includes(shelf) ? SHELF_ORDER.indexOf(shelf) : SHELF_ORDER.length);
  return [...groups.values()].sort(
    (a, b) =>
      Number(a.place !== RACK) - Number(b.place !== RACK) ||
      a.place.localeCompare(b.place) ||
      shelfRank(a.shelf) - shelfRank(b.shelf) ||
      a.shelf.localeCompare(b.shelf),
  );
}
