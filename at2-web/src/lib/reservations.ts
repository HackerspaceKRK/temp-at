import { apiPath } from "../config";

export interface ReservationEvent {
  id: number;
  phid: string;
  name: string;
  description: string;
  is_all_day: boolean;
  /** Unix epoch seconds. */
  start: number;
  /** Unix epoch seconds. */
  end: number;
  timezone: string;
  url: string;
  /** Display name of the event host (resolved from hostPHID); may be empty. */
  created_by: string;
}

/**
 * Fetch room reservations overlapping the [startSec, endSec) window (Unix epoch
 * seconds) from the tablet-only reservations endpoint. With no range it returns
 * the server's current day. Requires a tablet session (401 otherwise).
 */
export async function fetchReservations(
  startSec?: number,
  endSec?: number,
): Promise<ReservationEvent[]> {
  let path = "/api/v1/reservations";
  if (startSec !== undefined && endSec !== undefined) {
    path += `?start=${Math.floor(startSec)}&end=${Math.floor(endSec)}`;
  }
  const res = await fetch(apiPath(path));
  if (!res.ok) {
    throw new Error(`Failed to fetch reservations: ${res.status}`);
  }
  return (await res.json()) as ReservationEvent[];
}
