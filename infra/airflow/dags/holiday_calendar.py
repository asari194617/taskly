"""Check dates in an iCalendar holiday feed, including recurring events."""

from datetime import date, datetime, timedelta

from dateutil.rrule import rruleset, rrulestr


def _dates(component, name):
    properties = component.get(name)
    if properties is None:
        return
    if not isinstance(properties, list):
        properties = [properties]
    for prop in properties:
        for value in prop.dts:
            yield value.dt


def _as_datetime(value):
    if isinstance(value, datetime):
        return value
    return datetime.combine(value, datetime.min.time())


def is_calendar_holiday(calendar, target: date) -> bool:
    for event in calendar.walk("vevent"):
        start_property = event.get("dtstart")
        if start_property is None:
            continue
        start = _as_datetime(start_property.dt)
        end_property = event.get("dtend")
        duration = (end_property.dt - start_property.dt) if end_property else timedelta(days=1)
        if duration <= timedelta(0):
            continue

        occurrences = rruleset()
        rule = event.get("rrule")
        if rule is not None:
            occurrences.rrule(rrulestr(rule.to_ical().decode("ascii"), dtstart=start))
        else:
            occurrences.rdate(start)
        for value in _dates(event, "rdate"):
            occurrences.rdate(_as_datetime(value))
        for value in _dates(event, "exdate"):
            occurrences.exdate(_as_datetime(value))

        # DTSTART is inclusive; DTEND is exclusive. Check every occurrence
        # that could overlap the requested calendar day.
        target_start = datetime.combine(target, datetime.min.time(), tzinfo=start.tzinfo)
        target_end = target_start + timedelta(days=1)
        for occurrence in occurrences.between(target_start - duration, target_end, inc=True):
            if occurrence < target_end and occurrence + duration > target_start:
                return True
    return False
