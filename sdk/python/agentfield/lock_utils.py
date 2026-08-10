"""Lock acquisition with timeout to prevent indefinite hangs (#620).

The SDK uses threading.Lock / threading.RLock in several places. Without a
timeout, a deadlocked or contended lock causes the process to hang
indefinitely — the "stuck for hours" symptom reported in #620.

This module provides a context-manager wrapper that acquires with a timeout
and raises a clear error instead of hanging forever.
"""

from __future__ import annotations

import os
import threading
from contextlib import contextmanager
from typing import Union

from .logger import get_logger

logger = get_logger(__name__)

# Default timeout for lock acquisition (seconds). Long enough to never
# trip under normal contention, short enough to surface a real deadlock
# within minutes rather than hours. Configurable via the env var
# AGENTFIELD_LOCK_TIMEOUT_SECONDS.
DEFAULT_LOCK_TIMEOUT: float = float(
    os.environ.get("AGENTFIELD_LOCK_TIMEOUT_SECONDS", "30")
)


class LockTimeoutError(TimeoutError):
    """Raised when a lock cannot be acquired within the timeout period."""

    def __init__(self, lock_name: str, timeout: float):
        self.lock_name = lock_name
        self.timeout = timeout
        super().__init__(
            f"Failed to acquire lock '{lock_name}' within {timeout}s. "
            f"This may indicate a deadlock. Set AGENTFIELD_LOCK_TIMEOUT_SECONDS "
            f"to adjust the timeout."
        )


@contextmanager
def timed_lock(
    lock: Union[threading.Lock, threading.RLock],
    name: str = "unnamed",
    timeout: float = DEFAULT_LOCK_TIMEOUT,
):
    """Context manager that acquires a lock with a timeout.

    Usage:
        with timed_lock(self._lock, "result_cache"):
            # critical section

    Replaces bare ``with self._lock:`` to prevent indefinite hangs.

    Args:
        lock: The threading.Lock or threading.RLock to acquire.
        name: Human-readable name for error messages and logging.
        timeout: Maximum seconds to wait. Defaults to DEFAULT_LOCK_TIMEOUT.

    Raises:
        LockTimeoutError: If the lock cannot be acquired within the timeout.
    """
    acquired = lock.acquire(timeout=timeout)
    if not acquired:
        raise LockTimeoutError(name, timeout)
    try:
        yield
    finally:
        lock.release()
