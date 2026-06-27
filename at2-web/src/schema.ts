/* =============================
 * Enumerations (string literal unions)
 * ============================= */

export type EntityRepresentation =
  | "presence"
  | "camera_snapshot"
  | "temperature"
  | "humidity"
  | "light"
  | "fan"
  | "plug"
  | "power"
  | "co"
  | "gas"
  | "contact"
  | "printer";

/**
 * Relay states seen in sample: "OFF" plus null.
 * Future-proof: allow "ON" too.
 */
export type RelayState = "ON" | "OFF";

export type LocalizedName = {
  [locale: string]: string;
};

export interface SnapshotImage {
  url: string;
  width: number;
  height: number;
  media_type: string;
}

export interface SnapshotState {
  images: SnapshotImage[];
  low_res_preview?: string;
}

/* =============================
 * Discriminated entity variants
 * ============================= */

/**
 * Presence (person) entity - state currently always null in sample.
 */
export interface PresenceEntity {
  representation: "presence";
  id: string;
  localized_name: LocalizedName | null;
  type: "person";
  state: number | null; // Number of people
}

/**
 * Camera snapshot entity - state carries images.
 */
export interface CameraSnapshotEntity {
  representation: "camera_snapshot";
  id: string;
  type: "camera_snapshot";
  localized_name: LocalizedName | null;
  state: SnapshotState | null;
}

/**
 * Temperature sensor - currently null state (could later hold numeric readings).
 */
export interface TemperatureEntity {
  representation: "temperature";
  id: string;
  localized_name: LocalizedName | null;
  type: "temperature";
  state: number;
}

/**
 * Humidity sensor - currently null state.
 */
export interface HumidityEntity {
  representation: "humidity";
  type: "humidity";
  id: string;
  localized_name: LocalizedName | null;
  state: number;
}

/**
 * Power usage sensor.
 */
export interface PowerEntity {
  representation: "power";
  type: "power_usage";
  id: string;
  localized_name: LocalizedName | null;
  state: number;
}

/**
 * CO sensor (ppm).
 */
export interface CoEntity {
  representation: "co";
  type: "co";
  id: string;
  localized_name: LocalizedName | null;
  state: number;
}

/**
 * Gas sensor (LEL).
 */
export interface GasEntity {
  representation: "gas";
  type: "gas";
  id: string;
  localized_name: LocalizedName | null;
  state: number;
}

/**
 * Contact sensor/window sensor.
 * State: true (closed), false (open), or null (unknown).
 */
export interface ContactEntity {
  representation: "contact";
  type: "contact";
  id: string;
  localized_name: LocalizedName | null;
  state: boolean | null;
}

/**
 * Light / Fan / Plug relay entity.
 * Some have explicit "OFF", others null (unknown / not yet fetched).
 */
export interface RelayEntity {
  representation: "light" | "fan" | "plug";
  id: string;
  localized_name: LocalizedName | null;
  type: "relay";
  state: RelayState | null;
  prohibit_control?: boolean;
}

/**
 * Bambu Labs 3D printer state (mirrors the backend BambuPrinterState struct).
 */
export type PrinterStateValue =
  | "idle"
  | "printing"
  | "paused"
  | "finished"
  | "failed"
  | "offline";

export interface PrinterState {
  state: PrinterStateValue;
  progress: number; // percent 0-100
  remaining_time: number; // minutes
  filename: string;
  error_code: string;
  print_error: string; // 8-hex code (e.g. "0300800A"), "" when none
  print_error_text: string; // human-readable description, "" if none/unknown
  task_id: string;
  layer_num: number;
  total_layer_num: number;
  started_at: number; // unix millis, 0 if unknown
  finished_at: number; // unix millis, 0 while running
  nozzle_temp: number;
  nozzle_target: number;
  bed_temp: number;
  bed_target: number;
  chamber_temp: number;
  online: boolean;
}

export interface PrinterEntity {
  representation: "printer";
  type: "printer";
  id: string;
  localized_name: LocalizedName | null;
  state: PrinterState | null;
}

/**
 * Fallback for any entity that does not cleanly fit above
 * (keeps you resilient to future additions).
 */
export interface UnknownEntity {
  id: string;
  localized_name: LocalizedName | null;
  type: string;
  representation: string;
  state: any;
}

/**
 * Union of all known entity variants.
 * UnknownEntity at end for forward compatibility.
 */
export type Entity =
  | PresenceEntity
  | CameraSnapshotEntity
  | TemperatureEntity
  | HumidityEntity
  | PowerEntity
  | CoEntity
  | GasEntity
  | ContactEntity
  | RelayEntity
  | PrinterEntity
  | UnknownEntity;

export interface RoomState {
  id: string;
  localized_name: LocalizedName;
  exclude_from_entrance_tablet: boolean;
  people_count: number;
  latest_person_detected_at: string | null;
  voip_phone_number?: string;
  entities: Entity[];
}

export type RoomStates = RoomState[];

export interface UsageHeatmapDataPoint {
  startsAt: number;
  maxPeople: number;
  manHours: number;
  activeHours: number;
}

export interface UsageHeatmapResponse {
  dataPoints: UsageHeatmapDataPoint[];
}


