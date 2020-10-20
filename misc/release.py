#!/usr/bin/env python3

import argparse
import logging
import os
import subprocess
import time

logger = logging.getLogger(__name__)

#GOOS GOARCH DEB_HOST_ARCH
osArchArch = [
    ('linux', 'amd64', 'amd64'),
    ('linux', 'arm', 'armhf'),
    ('linux', 'arm64', 'arm64'),
    ('darwin', 'amd64', None),
]

def rstamp():
    return '{:08x}'.format(0xfffffffff - int(time.time()))

def compile(goos=None, goarch=None, ldflags=None, odir=None, rt=None):
    env = dict(os.environ)
    env['CGO_ENABLED'] = '0'
    if goos is not None:
        env['GOOS'] = goos
    if goarch is not None:
        env['GOARCH'] = goarch
    cmd = ['go', 'build']
    if ldflags is not None:
        cmd.append(ldflags)
    if odir and goos and goarch:
        if rt is None:
            rt = rstamp()
        odir = os.path.abspath(os.path.join(odir, goos, goarch, rt))
        os.makedirs(odir, exist_ok=True)
        opath = os.path.join(odir, 'tracecollector')
        cmd.append('-o')
        cmd.append(opath)
    subprocess.run(cmd, cwd='cmd/tracecollector', env=env).check_returncode()

def main():
    start = time.time()
    ap = argparse.ArgumentParser()
    ap.add_argument('-o', '--outdir', help='The output directory for the build assets', type=str, default='release')
    ap.add_argument('--verbose', action='store_true', default=False)
    args = ap.parse_args()

    if args.verbose:
        logging.basicConfig(level=logging.DEBUG)
    else:
        logging.basicConfig(level=logging.INFO)
    outdir = args.outdir
    rt = rstamp()
    for goos, goarch, debarch in osArchArch:
        logger.info('GOOS=%s GOARCH=%s DEB_HOST_ARCH=%s', goos, goarch, debarch)
        compile(goos, goarch, ldflags=None, odir=outdir, rt=rt)
    dt = time.time() - start
    logger.info('done %0.1fs', dt)
    return

if __name__ == '__main__':
    main()
