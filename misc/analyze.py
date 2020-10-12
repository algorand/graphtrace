#!/usr/bin/env python3

import json
import logging
import sys

def da(d, k, v):
    l = d.get(k)
    if l is None:
        d[k] = [v]
    else:
        l.append(v)

def analyze(fin):
    # byEvent[event] = [(micros, addr, port), ...]
    byEvent = {}
    for line in fin:
        if not line:
            continue
        if line[0] == '#':
            continue
        line = line.strip()
        if not line:
            continue
        parts = line.split('\t')
        micros = int(parts[0])
        addr = parts[1]
        port = parts[2]
        event = parts[3]
        da(byEvent, event, (micros, addr, port))

    for event, el in byEvent.items():
        # el.sort() # ?
        mint = None
        maxt = None
        count = 0
        for micros, addr, port in el:
            if mint is None or micros < mint:
                mint = micros
            if maxt is None or micros > maxt:
                maxt = micros
            count += 1
        rec = {
            'event': event,
            'mint': mint,
            'maxt': maxt,
            'dt': maxt-mint,
            'count': count,
        }
        print(json.dumps(rec, sort_keys=True))

def main():
    import argparse
    ap = argparse.ArgumentParser()
    ap.add_argument('inputs', nargs='*')
    ap.add_argument('--verbose', default=False, action='store_true')
    args = ap.parse_args()
    if args.verbose:
        logging.basicConfig(level=logging.DEBUG)
    else:
        logging.basicConfig(level=logging.INFO)
    if len(args.inputs) == 0:
        analyze(sys.stdin)
    else:
        for fname in args.inputs:
            with open(fname, 'r') as fin:
                analyze(fin)
    return 0

if __name__ == '__main__':
    sys.exit(main())
