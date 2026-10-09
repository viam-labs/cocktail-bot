import * as VIAM from "@viamrobotics/sdk";

export const ORG_ID = "e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb";
export const LOCATION_ID = "oeq47g5p1m";

export interface Machine {
  id: string;
  name: string;
  locationName: string;
  online: boolean;
  lastOnline: Date | null;
  mainPartId: string | null;
}

export async function listMachines(client: VIAM.ViamClient): Promise<Machine[]> {
  const summaries = await client.appClient.listMachineSummaries(ORG_ID, [], [LOCATION_ID]);
  const machines: Machine[] = [];
  for (const location of summaries) {
    for (const m of location.machineSummaries) {
      const mainPart = m.partSummaries.find((p) => p.isMainPart) ?? m.partSummaries[0];
      machines.push({
        id: m.machineId,
        name: m.machineName,
        locationName: location.locationName,
        online: mainPart?.onlineState === VIAM.appApi.OnlineState.ONLINE,
        lastOnline: mainPart?.lastOnline?.toDate() ?? null,
        mainPartId: mainPart?.partId ?? null,
      });
    }
  }
  machines.sort((a, b) => a.name.localeCompare(b.name));
  return machines;
}
