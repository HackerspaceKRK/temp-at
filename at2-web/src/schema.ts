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
  | "power";

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
  | RelayEntity
  | UnknownEntity;

export interface RoomState {
  id: string;
  localized_name: LocalizedName;
  people_count: number;
  latest_person_detected_at: string | null;
  entities: Entity[];
}

export type RoomStates = RoomState[];
