import ast
import sys
import unittest
from datetime import date, datetime, timezone
from pathlib import Path

from icalendar import Calendar


DAGS = Path(__file__).resolve().parents[1] / "dags"
sys.path.insert(0, str(DAGS))
from school_calendar import JST, event_overlaps_date


def calendar_event(start, end="", extra=""):
    calendar = Calendar.from_ical(f"""BEGIN:VCALENDAR
BEGIN:VEVENT
SUMMARY:School event
DESCRIPTION:DAY_SHIFT_OFF
DTSTART{start}
{end}
{extra}
END:VEVENT
END:VCALENDAR
""")
    return calendar, calendar.walk("vevent")[0]


def dag_function(filename, name, calendar):
    source = DAGS / filename
    tree = ast.parse(source.read_text())
    function = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == name)
    namespace = {
        "SCHOOL_CAL_URL": "test://school",
        "DISCORD_WEBHOOK_URL": "",
        "fetch_ics": lambda _: calendar,
        "event_overlaps_date": event_overlaps_date,
        "JST": JST,
        "datetime": datetime,
        "date": date,
    }
    exec(compile(ast.Module(body=[function], type_ignores=[]), str(source), "exec"), namespace)
    return namespace[name]


class SchoolCalendarTest(unittest.TestCase):
    def check_both_dags(self, start, end, expected_dates, extra=""):
        calendar, event = calendar_event(start, end, extra)
        is_off = dag_function("day_shift_index_increment.py", "is_day_shift_off", calendar)
        list_events = dag_function("day_shift_notify_0830.py", "list_events_for_date", calendar)
        for target, expected in expected_dates.items():
            with self.subTest(target=target):
                self.assertEqual(event_overlaps_date(event, target), expected)
                self.assertEqual(is_off(datetime.combine(target, datetime.min.time())), expected)
                self.assertEqual(list_events(target), ["School event"] if expected else [])

    def test_timed_event_ends_at_noon_next_day(self):
        self.check_both_dags(":20260921T130000", "DTEND:20260922T120000", {
            date(2026, 9, 21): True,
            date(2026, 9, 22): True,
            date(2026, 9, 23): False,
        })

    def test_midnight_end_is_exclusive(self):
        self.check_both_dags(":20260921T130000", "DTEND:20260922T000000", {
            date(2026, 9, 21): True,
            date(2026, 9, 22): False,
        })

    def test_all_day_end_is_exclusive(self):
        self.check_both_dags(";VALUE=DATE:20260921", "DTEND;VALUE=DATE:20260923", {
            date(2026, 9, 21): True,
            date(2026, 9, 22): True,
            date(2026, 9, 23): False,
        })

    def test_short_duration_crosses_midnight(self):
        self.check_both_dags(":20260921T230000", "", {
            date(2026, 9, 21): True,
            date(2026, 9, 22): True,
            date(2026, 9, 23): False,
        }, "DURATION:PT2H")

    def test_utc_event_uses_jst_date_boundaries(self):
        self.check_both_dags(":20260921T140000Z", "DTEND:20260921T160000Z", {
            date(2026, 9, 21): True,
            date(2026, 9, 22): True,
        })

    def test_increment_converts_aware_input_to_jst(self):
        calendar, _ = calendar_event(":20260922T080000", "DTEND:20260922T090000")
        is_off = dag_function("day_shift_index_increment.py", "is_day_shift_off", calendar)
        self.assertTrue(is_off(datetime(2026, 9, 21, 23, tzinfo=timezone.utc)))


if __name__ == "__main__":
    unittest.main()
