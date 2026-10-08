import type { JsonValue } from "@bufbuild/protobuf";
import type { ViamClient, RobotClient } from "@viamrobotics/sdk";

export interface ViamConnection {
  viamClient: ViamClient;
  robotClient: RobotClient;
  machineId: string;
  hostname: string;
  isDev: boolean;
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
  const robot = await conn.viamClient.appClient.getRobot(conn.machineId);
  return robot?.name ?? "";
}
