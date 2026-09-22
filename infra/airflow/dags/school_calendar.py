"""Shared day overlap rules for school calendar events."""

from datetime import date, datetime, time, timedelta, timezone


JST = timezone(timedelta(hours=9))


def _in_jst(value: datetime) -> datetime:
    # A floating DATE-TIME has no timezone; the school calendar uses local time.
    if value.tzinfo is None:
        return value.replace(tzinfo=JST)
    return value.astimezone(JST)


def event_overlaps_date(event, target_date: date) -> bool:
    """Return whether a VEVENT occupies any time on a JST calendar day.

    DTSTART is inclusive and DTEND is exclusive for both DATE and DATE-TIME.
    A DATE without DTEND covers one day; a DATE-TIME without an end is an
    instant on its start day.
    """
    start_property = event.get("dtstart")
    if start_property is None:
        return False

    start = start_property.dt
    end_property = event.get("dtend")
    duration_property = event.get("duration")
    if isinstance(start, datetime):
        start_time = _in_jst(start)
        if end_property is not None:
            end = end_property.dt
            end_time = _in_jst(end) if isinstance(end, datetime) else datetime.combine(end, time.min, JST)
        elif duration_property is not None:
            duration = getattr(duration_property, "dt", duration_property)
            end_time = start_time + duration
        else:
            return start_time.date() == target_date
    else:
        start_time = datetime.combine(start, time.min, JST)
        if end_property is not None:
            end = end_property.dt
            end_time = _in_jst(end) if isinstance(end, datetime) else datetime.combine(end, time.min, JST)
        elif duration_property is not None:
            duration = getattr(duration_property, "dt", duration_property)
            end_time = start_time + duration
        else:
            end_time = start_time + timedelta(days=1)

    day_start = datetime.combine(target_date, time.min, JST)
    day_end = day_start + timedelta(days=1)
    return start_time < end_time and start_time < day_end and end_time > day_start
