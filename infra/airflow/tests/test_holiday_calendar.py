import sys
import unittest
from datetime import date
from pathlib import Path

from icalendar import Calendar

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "dags"))
from holiday_calendar import is_calendar_holiday


class HolidayCalendarTest(unittest.TestCase):
    def test_recurrence_additions_and_exclusions(self):
        calendar = Calendar.from_ical("""BEGIN:VCALENDAR
BEGIN:VEVENT
DTSTART;VALUE=DATE:20030915
DTEND;VALUE=DATE:20030916
RRULE:FREQ=YEARLY;BYMONTH=9;BYDAY=3MO;UNTIL=20271231
EXDATE;VALUE=DATE:20250915
RDATE;VALUE=DATE:20260922
END:VEVENT
END:VCALENDAR
""")
        self.assertTrue(is_calendar_holiday(calendar, date(2026, 9, 21)))
        self.assertTrue(is_calendar_holiday(calendar, date(2026, 9, 22)))
        self.assertFalse(is_calendar_holiday(calendar, date(2025, 9, 15)))
        self.assertFalse(is_calendar_holiday(calendar, date(2026, 9, 24)))

    def test_multi_day_event_has_exclusive_end(self):
        calendar = Calendar.from_ical("""BEGIN:VCALENDAR
BEGIN:VEVENT
DTSTART;VALUE=DATE:20260921
DTEND;VALUE=DATE:20260923
END:VEVENT
END:VCALENDAR
""")
        self.assertTrue(is_calendar_holiday(calendar, date(2026, 9, 22)))
        self.assertFalse(is_calendar_holiday(calendar, date(2026, 9, 23)))


if __name__ == "__main__":
    unittest.main()
