"""Pure recipient selection for the day-shift notification DAG."""


def select_day_shift_recipients(all_users, blocked_map, pref_map, dev_map, subscribed_ids):
    enabled, blocked, email, push = [], [], [], []
    for user in all_users:
        if user.get("id") is None:
            continue
        uid = int(user["id"])
        if blocked_map.get(uid, 0) > 0:
            blocked.append(user)
            continue
        if pref_map.get(uid, 1) == 0:
            continue
        enabled.append(user)
        if dev_map.get((uid, "EMAIL"), 1) == 1 and (user.get("email") or "").strip():
            email.append(user)
        if dev_map.get((uid, "WEBPUSH"), 0) == 1 and uid in subscribed_ids:
            push.append(user)
    return enabled, blocked, email, push
