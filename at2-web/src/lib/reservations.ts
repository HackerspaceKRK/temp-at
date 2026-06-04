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
  /** Display name of the creator; currently always empty (see backend TODO). */
  created_by: string;
}

/**
 * Fetch today's room reservations from the tablet-only reservations endpoint.
 * Requires a tablet session (the endpoint returns 401 otherwise).
 */
export async function fetchReservations(): Promise<ReservationEvent[]> {
  const res = await fetch(apiPath("/api/v1/reservations"));
  if (!res.ok) {
    throw new Error(`Failed to fetch reservations: ${res.status}`);
  }
  return (await res.json()) as ReservationEvent[];
}
