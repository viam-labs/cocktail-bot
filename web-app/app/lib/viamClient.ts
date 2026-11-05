import type { JsonValue } from "@bufbuild/protobuf";
import type { ViamClient, RobotClient } from "@viamrobotics/sdk";
import type { Recipe, RecipeStep } from "./recipes";
import type { IngredientStock, Inventory, WhiskeyProfile } from "./inventory";

const BARTENDER_SERVICE_NAME = "bartender";

export interface ViamConnection {
  viamClient: ViamClient;
  robotClient: RobotClient;
  machineId: string;
  hostname: string;
  isDev: boolean;
}

// Dev mode returns mock data so the app runs without a real machine or Viam
// auth. Enabled when the userToken cookie isn't present (localhost), or forced
// via ?mock=1 / ?mock=0.
function isDevMode(): boolean {
  if (typeof window === "undefined") return false;
  const params = new URLSearchParams(window.location.search);
  const mock = params.get("mock");
  if (mock === "1" || mock === "true") return true;
  if (mock === "0" || mock === "false") return false;
  return !hasUserToken();
}

function hasUserToken(): boolean {
  if (typeof document === "undefined") return false;
  for (const part of document.cookie.split(";")) {
    if (part.trim().startsWith("userToken=")) return true;
  }
  return false;
}

const DEV_RECIPES: Recipe[] = [
  {
    id: "espresso_martini",
    name: "Espresso Martini",
    pours: [
      { ingredient: "vodka", oz: 2 },
      { ingredient: "coffee-liquor", oz: 1 },
    ],
  },
];

const DEV_INVENTORY: Inventory = {
  ingredients: {
    vodka: { in_stock: true, unopened: 2, slot: "slot-a-front" },
    "coffee-liquor": { in_stock: true, unopened: 0, slot: "slot-b-front" },
    "lime-juice": { in_stock: true, unopened: 1, refrigerated: true, slot: "slot-c-front" },
    "dry-vermouth": { in_stock: true, unopened: 0, refrigerated: true, slot: "slot-d-back" },
    "simple-syrup": { in_stock: false, unopened: 3, refrigerated: true },
  },
  slots: [
    { id: "slot-a-front", ingredient: "vodka" },
    { id: "slot-b-front", ingredient: "coffee-liquor" },
    { id: "slot-c-front", ingredient: "lime-juice" },
    { id: "slot-d-back", ingredient: "dry-vermouth" },
  ],
};

const DEV_POUR_MENUS: import("./pour_menus").PourMenu[] = [
  {
    id: "whiskey-bar",
    name: "Whiskey bar",
    category: "Whiskey",
    on_menu: true,
    sizes: [{ label: "Taste", oz: 1 }, { label: "Pour", oz: 2 }],
    default_size: "Taste",
    serves: [
      { label: "Neat", glass: "rocks" },
      { label: "On the rocks", glass: "rocks", with_ice: true },
    ],
    traits: ["Peat", "Smoke", "Smoothness", "Sweetness", "Fruitiness", "Heat", "Body"],
  },
];

// Simulated verb duration so dev-mode progress bars feel alive.
const DEV_VERB_DELAY_MS = 1500;

async function devDelay(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, DEV_VERB_DELAY_MS));
}

let sdkCache: {
  createViamClient: typeof import("@viamrobotics/sdk").createViamClient;
  GenericServiceClient: typeof import("@viamrobotics/sdk").GenericServiceClient;
  Cookies: typeof import("js-cookie").default;
} | null = null;

async function loadSDK() {
  if (sdkCache) return sdkCache;
  const [viamSdk, cookies] = await Promise.all([
    import("@viamrobotics/sdk"),
    import("js-cookie"),
  ]);
  sdkCache = {
    createViamClient: viamSdk.createViamClient,
    GenericServiceClient: viamSdk.GenericServiceClient,
    Cookies: cookies.default,
  };
  return sdkCache;
}

/**
 * Issue a DoCommand against a generic service on the machine. The SDK is
 * loaded lazily so each call builds its client from the loaded module rather
 * than holding one; the backend returns plain JSON maps that the SDK types
 * only as `unknown`, so callers name the shape they expect via T.
 */
export async function doCommand<T>(
  conn: ViamConnection,
  serviceName: string,
  command: Record<string, JsonValue>,
): Promise<T> {
  const sdk = await loadSDK();
  const svc = new sdk.GenericServiceClient(conn.robotClient, serviceName);
  return (await svc.doCommand(command)) as unknown as T;
}

export async function connectToViam(partId: string): Promise<ViamConnection> {
  if (isDevMode()) {
    console.log("[dev] using mock Viam connection");
    return {
      viamClient: {} as ViamClient,
      robotClient: {} as RobotClient,
      machineId: "dev-machine",
      hostname: "localhost",
      isDev: true,
    };
  }

  if (!partId) {
    throw new Error("connectToViam: partId is required");
  }

  const sdk = await loadSDK();

  const raw = sdk.Cookies.get("userToken");
  if (!raw) {
    throw new Error('No "userToken" cookie found');
  }
  const { access_token } = JSON.parse(raw) as { access_token: string };

  const viamClient = await sdk.createViamClient({
    credentials: {
      type: "access-token",
      payload: access_token,
    },
  });

  const partResp = await viamClient.appClient.getRobotPart(partId);
  const part = partResp.part;
  if (!part) {
    throw new Error(`getRobotPart(${partId}) returned no part`);
  }

  const robotClient = await viamClient.connectToMachine({
    host: part.fqdn,
  });

  return {
    viamClient,
    robotClient,
    machineId: part.robot,
    hostname: part.fqdn,
    isDev: false,
  };
}

export async function getMachineName(conn: ViamConnection): Promise<string> {
  if (conn.isDev) return "dev-machine";
  const robot = await conn.viamClient.appClient.getRobot(conn.machineId);
  return robot?.name ?? "";
}

export async function getRecipes(conn: ViamConnection): Promise<Recipe[]> {
  if (conn.isDev) return DEV_RECIPES;
  const resp = await doCommand<{ recipes: Recipe[] }>(
    conn,
    BARTENDER_SERVICE_NAME,
    { get_recipes: true },
  );
  return resp.recipes ?? [];
}

export interface OrderStatus {
  is_busy: boolean;
  drink?: string;
  current_step?: string;
  started_at?: string;
  elapsed_ms?: number;
  step_history?: { step: string; started_at: string }[];
  last_outcome?: { drink: string; ended_at: string; error?: string };
}

export async function getStatus(conn: ViamConnection): Promise<OrderStatus> {
  if (conn.isDev) return { is_busy: false };
  return doCommand<OrderStatus>(conn, BARTENDER_SERVICE_NAME, { get_status: true });
}

export async function getBottles(conn: ViamConnection): Promise<string[]> {
  if (conn.isDev) return ["vodka", "coffee-liquor", "espresso"];
  const resp = await doCommand<{ bottles: string[] }>(conn, BARTENDER_SERVICE_NAME, { get_bottles: true });
  return (resp.bottles ?? []).slice().sort();
}

export async function getInventory(conn: ViamConnection): Promise<Inventory> {
  if (conn.isDev) return structuredClone(DEV_INVENTORY);
  const resp = await doCommand<{ inventory: Inventory }>(
    conn,
    BARTENDER_SERVICE_NAME,
    { get_inventory: true },
  );
  return resp.inventory ?? { ingredients: {} };
}

export async function updateInventoryItem(
  conn: ViamConnection,
  ingredient: string,
  inStock: boolean,
): Promise<{ ingredient: string; in_stock: boolean }> {
  if (conn.isDev) {
    DEV_INVENTORY.ingredients[ingredient] = { ...DEV_INVENTORY.ingredients[ingredient], in_stock: inStock };
    return { ingredient, in_stock: inStock };
  }
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    update_inventory_item: { ingredient, in_stock: inStock },
  });
}

function devKnownStock(ingredient: string): IngredientStock {
  const stock = DEV_INVENTORY.ingredients[ingredient];
  if (stock) return stock;
  if (DEV_INVENTORY.slots?.some((s) => s.ingredient === ingredient)) return { in_stock: false };
  throw new Error(`unknown ingredient "${ingredient}"`);
}

export async function adjustUnopened(
  conn: ViamConnection,
  ingredient: string,
  delta: number,
): Promise<{ ingredient: string; unopened: number }> {
  if (conn.isDev) {
    const stock = devKnownStock(ingredient);
    const unopened = Math.max((stock.unopened ?? 0) + delta, 0);
    DEV_INVENTORY.ingredients[ingredient] = { ...stock, unopened };
    return { ingredient, unopened };
  }
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    adjust_unopened: { ingredient, delta },
  });
}

export async function setUnopened(
  conn: ViamConnection,
  ingredient: string,
  count: number,
): Promise<{ ingredient: string; unopened: number }> {
  if (conn.isDev) {
    const stock = devKnownStock(ingredient);
    DEV_INVENTORY.ingredients[ingredient] = { ...stock, unopened: count };
    return { ingredient, unopened: count };
  }
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    set_unopened: { ingredient, count },
  });
}

export async function updateRefrigerated(
  conn: ViamConnection,
  ingredient: string,
  refrigerated: boolean,
): Promise<{ ingredient: string; refrigerated: boolean }> {
  if (conn.isDev) {
    const stock = devKnownStock(ingredient);
    DEV_INVENTORY.ingredients[ingredient] = { ...stock, refrigerated };
    return { ingredient, refrigerated };
  }
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    update_refrigerated: { ingredient, refrigerated },
  });
}

export async function deleteInventoryItem(
  conn: ViamConnection,
  ingredient: string,
): Promise<{ ingredient: string; deleted: boolean }> {
  if (conn.isDev) {
    delete DEV_INVENTORY.ingredients[ingredient];
    DEV_INVENTORY.slots = DEV_INVENTORY.slots?.map((s) => (s.ingredient === ingredient ? { id: s.id } : s));
    return { ingredient, deleted: true };
  }
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    delete_inventory_item: { ingredient },
  });
}

export async function updateSlotAssignment(
  conn: ViamConnection,
  slot: string,
  ingredient: string,
): Promise<{ slot: string; ingredient: string }> {
  if (conn.isDev) return { slot, ingredient };
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    update_slot_assignment: { slot, ingredient },
  });
}

export async function clearSlotAssignment(
  conn: ViamConnection,
  slot: string,
): Promise<{ slot: string }> {
  if (conn.isDev) return { slot };
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    clear_slot_assignment: { slot },
  });
}

export async function updatePourerOz(
  conn: ViamConnection,
  ingredient: string,
  oz: number,
): Promise<{ ingredient: string; oz: number }> {
  if (conn.isDev) return { ingredient, oz };
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    update_pourer_oz: { ingredient, oz },
  });
}

export async function updateWhiskeyProfile(
  conn: ViamConnection,
  ingredient: string,
  profile: WhiskeyProfile,
): Promise<{ ingredient: string; profile: WhiskeyProfile }> {
  if (conn.isDev) return { ingredient, profile };
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    update_whiskey_profile: { ingredient, profile: profile as unknown as import("@bufbuild/protobuf").JsonValue },
  });
}

export async function clearWhiskeyProfile(
  conn: ViamConnection,
  ingredient: string,
): Promise<{ ingredient: string; cleared: boolean }> {
  if (conn.isDev) return { ingredient, cleared: true };
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    clear_whiskey_profile: { ingredient },
  });
}

export async function updateCategory(
  conn: ViamConnection,
  ingredient: string,
  category: string,
): Promise<{ ingredient: string; category: string }> {
  if (conn.isDev) return { ingredient, category };
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    update_category: { ingredient, category },
  });
}

export async function updateRecipes(
  conn: ViamConnection,
  recipes: Recipe[],
): Promise<{ count: number }> {
  if (conn.isDev) {
    DEV_RECIPES.splice(0, DEV_RECIPES.length, ...recipes);
    return { count: recipes.length };
  }
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    update_recipes: { recipes: recipes as unknown as import("@bufbuild/protobuf").JsonValue },
  });
}

export async function getPourMenus(conn: ViamConnection): Promise<import("./pour_menus").PourMenu[]> {
  if (conn.isDev) return DEV_POUR_MENUS;
  const resp = await doCommand<{ pour_menus: import("./pour_menus").PourMenu[] }>(
    conn,
    BARTENDER_SERVICE_NAME,
    { get_pour_menus: true },
  );
  return resp.pour_menus ?? [];
}

export async function updatePourMenus(
  conn: ViamConnection,
  menus: import("./pour_menus").PourMenu[],
): Promise<{ count: number }> {
  if (conn.isDev) {
    DEV_POUR_MENUS.splice(0, DEV_POUR_MENUS.length, ...menus);
    return { count: menus.length };
  }
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    update_pour_menus: { pour_menus: menus as unknown as import("@bufbuild/protobuf").JsonValue },
  });
}

export async function pourIntoShaker(
  conn: ViamConnection,
  bottle: string,
  oz: number,
): Promise<void> {
  if (conn.isDev) return devDelay();
  await doCommand(conn, BARTENDER_SERVICE_NAME, {
    pour_into_shaker: { bottle, oz },
  });
}

export async function dispenseIce(
  conn: ViamConnection,
  station: string,
  dwellMs: number,
): Promise<void> {
  if (conn.isDev) return devDelay();
  await doCommand(conn, BARTENDER_SERVICE_NAME, {
    dispense_ice: { station, dwell_ms: dwellMs },
  });
}

export async function mix(
  conn: ViamConnection,
  station: string,
  dwellMs: number,
): Promise<void> {
  if (conn.isDev) return devDelay();
  await doCommand(conn, BARTENDER_SERVICE_NAME, {
    mix: { station, dwell_ms: dwellMs },
  });
}

export async function pourFromShaker(
  conn: ViamConnection,
  station: string,
  pourMs: number,
): Promise<void> {
  if (conn.isDev) return devDelay();
  await doCommand(conn, BARTENDER_SERVICE_NAME, {
    pour_from_shaker: { station, pour_ms: pourMs },
  });
}

async function awaitOrder(conn: ViamConnection, startedAt: string): Promise<void> {
  const firedAt = new Date(startedAt).getTime();
  while (true) {
    await new Promise((r) => setTimeout(r, 1000));
    const status = await getStatus(conn);
    if (status.is_busy) continue;
    const outcome = status.last_outcome;
    if (outcome && new Date(outcome.ended_at).getTime() >= firedAt) {
      if (outcome.error) throw new Error(outcome.error);
      return;
    }
  }
}

export async function makeCocktail(
  conn: ViamConnection,
  drinkId: string,
  quantity: number = 1,
): Promise<void> {
  const q = Math.max(1, Math.min(4, Math.floor(quantity)));
  if (conn.isDev) {
    const recipe = DEV_RECIPES.find((r) => r.id === drinkId);
    const phases = (recipe?.pours?.length ?? 0) + 5;
    await new Promise((resolve) => setTimeout(resolve, DEV_VERB_DELAY_MS * phases * q));
    return;
  }
  const resp = await doCommand<{ started_at: string }>(conn, BARTENDER_SERVICE_NAME, {
    make_cocktail: { drink_id: drinkId, quantity: q },
  });
  await awaitOrder(conn, resp.started_at);
}

export async function makeInlineOrder(
  conn: ViamConnection,
  recipe: { name: string; steps: RecipeStep[] },
): Promise<void> {
  if (conn.isDev) {
    await new Promise((resolve) => setTimeout(resolve, DEV_VERB_DELAY_MS * (recipe.steps.length + 2)));
    return;
  }
  const resp = await doCommand<{ started_at: string }>(conn, BARTENDER_SERVICE_NAME, {
    make_cocktail: { recipe: recipe as unknown as import("@bufbuild/protobuf").JsonValue },
  });
  await awaitOrder(conn, resp.started_at);
}
