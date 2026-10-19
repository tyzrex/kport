#!/usr/bin/env python3
"""Exercise an already-built kport in a real PTY (Unix; Python stdlib only).
Usage: python3 scripts/check-tui.py /path/to/kport
"""
import fcntl
import os
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time

binary = os.path.abspath(sys.argv[1])
with tempfile.TemporaryDirectory(prefix="kport-tui-") as directory:
    listener = subprocess.Popen(
        [sys.executable, "-u", "-c",
         "import socket,time; s=socket.socket(); s.bind(('127.0.0.1',0)); "
         "s.listen(); print(s.getsockname()[1],flush=True); time.sleep(120)"],
        stdout=subprocess.PIPE, text=True, cwd=directory)
    master, slave = os.openpty()
    before = termios.tcgetattr(slave)
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 30, 120, 0, 0))
    port = listener.stdout.readline().strip()
    process = subprocess.Popen(
        [binary, "--ui", port, "--interval", "250ms"],
        stdin=slave, stdout=slave, stderr=slave, cwd=directory,
        env={**os.environ, "HOME": directory, "TERM": "xterm-256color"})
    output = bytearray()

    def until(fragment, timeout=12):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if fragment.encode() in output:
                return
            readable, _, _ = select.select([master], [], [], 0.1)
            if readable:
                output.extend(os.read(master, 65536))
        raise AssertionError(f"Missing {fragment!r}; terminal output:\n{output.decode(errors='replace')}")

    def send(text):
        output.clear()
        os.write(master, text.encode())

    try:
        until("Ready.")
        send("/fastify")
        until("fastify_")
        assert listener.poll() is None, "typing f must not kill"
        send("\x1b")
        until("search: ")
        send("\x1b[200~Fxyq猫\x1b[201~")
        until("Fxyq猫_")
        assert listener.poll() is None and process.poll() is None, "paste triggered shortcut"
        send("\x1b")
        until("search: ")
        send("\r")
        until("Command:")
        send("x")
        until("[y] Confirm")
        send("n")
        until("Action canceled")
        assert listener.poll() is None, "canceled action stopped listener"
        send("xy")
        until("Stopped PID")
        listener.wait(timeout=5)
        assert process.poll() is None, "dashboard exited after action"
        # Resize to a short/narrow terminal and back without leaving raw mode.
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 8, 25, 0, 0))
        output.clear()
        until("Terminal too small")
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 30, 120, 0, 0))
        output.clear()
        until("kport | live port manager")
        send("q")
        assert process.wait(timeout=5) == 0
        assert termios.tcgetattr(slave) == before, "terminal mode not restored"
        print("PASS: search, Unicode paste, details, confirmation/cancel, stop, persistence, resize, restoration")
    finally:
        if process.poll() is None:
            process.send_signal(signal.SIGTERM)
            process.wait(timeout=5)
        if listener.poll() is None:
            listener.terminate()
        listener.wait(timeout=5)
        os.close(master)
        os.close(slave)
