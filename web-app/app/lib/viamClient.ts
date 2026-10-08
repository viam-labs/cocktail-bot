import type { JsonValue } from "@bufbuild/protobuf";
import type { ViamClient, RobotClient } from "@viamrobotics/sdk";
import type { Recipe } from "./recipes";
import type { Inventory } from "./inventory";

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
    steps: [
      { verb: "pour_into_shaker", bottle: "vodka", oz: 2 },
      { verb: "pour_into_shaker", bottle: "coffee-liquor", oz: 1 },
      { verb: "dispense_ice", station: "ice-station", dwell_ms: 3000 },
      { verb: "mix", station: "mixer", dwell_ms: 10000 },
      { verb: "pour_from_shaker", station: "serving", pour_ms: 5000 },
    ],
  },
];

const DEV_INVENTORY: Inventory = {
  ingredients: {
    vodka: { in_stock: true },
    "coffee-liquor": { in_stock: true },
  },
};

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
    DEV_INVENTORY.ingredients[ingredient] = { in_stock: inStock };
    return { ingredient, in_stock: inStock };
  }
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    update_inventory_item: { ingredient, in_stock: inStock },
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

export async function makeCocktail(
  conn: ViamConnection,
  drinkId: string,
): Promise<{ duration_ms: number }> {
  if (conn.isDev) {
    const recipe = DEV_RECIPES.find((r) => r.id === drinkId);
    const stepCount = recipe?.steps.length ?? 1;
    await new Promise((resolve) => setTimeout(resolve, DEV_VERB_DELAY_MS * stepCount));
    return { duration_ms: DEV_VERB_DELAY_MS * stepCount };
  }
  return doCommand(conn, BARTENDER_SERVICE_NAME, {
    make_cocktail: { drink_id: drinkId },
  });
}
