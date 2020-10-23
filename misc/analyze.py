#!/usr/bin/env python3
#
# usage:
#  python3 misc/analyze.py trace.tsv
#
# TODO: graph recs by time to show speed during different phases of a test

import base64
import json
import logging
import statistics
import sys

def da(d, k, v):
    l = d.get(k)
    if l is None:
        d[k] = [v]
    else:
        l.append(v)


def reportByEventType(byEventType, reportout):
    for et, recs in byEventType.items():
        reportOneEventType(et, recs, reportout)
def reportOneEventType(et, recs, reportout):
    counts = [r['count'] for r in recs]
    countavg = statistics.mean(counts)
    countstd = statistics.stdev(counts, countavg)
    lowcount = countavg # - (countstd * 1)
    dts = []
    for r in recs:
        if r['count'] > lowcount:
            dts.append(r['dt'])
    reportout.write('{}: {} recs, counts (min={}, avg={}, max={}, std={}), {} above {}\n'.format(et, len(recs), min(counts), countavg, max(counts), countstd, len(dts), lowcount))
    dtavg = statistics.mean(dts)
    dtstd = statistics.stdev(dts, dtavg)
    reportout.write('{}: times µs (min={}, avg={}, max={}, std={}\n'.format(et, min(dts), dtavg, max(dts), dtstd))
    return


def analyze(fin, recout, reportout):
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

    byEventType = {}
    reccount = 0

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
        try:
            eb = base64.b64decode(event)
            if eb.startswith(b'prop'):
                da(byEventType, 'prop', rec)
        except:
            pass
        recout.write(json.dumps(rec, sort_keys=True) + '\n')
        reccount += 1
    reportout.write('{} event records\n'.format(reccount))
    reportByEventType(byEventType, reportout)

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
    recout = sys.stdout
    reportout = sys.stdout
    if len(args.inputs) == 0:
        analyze(sys.stdin, recout, reportout)
    else:
        for fname in args.inputs:
            with open(fname, 'r') as fin:
                analyze(fin, recout, reportout)
    return 0

if __name__ == '__main__':
    sys.exit(main())
