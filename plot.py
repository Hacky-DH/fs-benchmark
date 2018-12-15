#!/usr/bin/env python
import sys
import os
import tarfile
import re
import pandas as pd
from pandas.plotting import table
import matplotlib.pyplot as plt
import numpy as np
import argparse

URL='http://'
KBS_PATTERN = re.compile(r'".*"\s*([0-9]+\.[0-9]+).*', re.IGNORECASE)
NAME_PATTERN = re.compile(r'iozone-r([0-9]+)-t([0-9]+)', re.IGNORECASE)
COLUMNS = ['bs','thread','write','rewrite','read','reread','randread','randwrite']
METRIX = ['write','read','randread','randwrite']

def download_untar(url):
    name = os.path.basename(url)
    if not os.path.exists(name):
        import requests
        r = requests.get(url)
        if r.status_code != 200:
            print('download err code: {}'.format(r.status_code))
            return None
        print('download {} OK'.format(url))
        with open(name,'wb') as fd:
            for chunk in r.iter_content(chunk_size=8192):
                fd.write(chunk)
    try:
        tar = tarfile.open(name, mode='r:gz')
        return [ti for ti in tar if ti.isfile() and ti.name.endswith('.log')], tar
    except tarfile.TarError as e:
        print('tar file error:', e, file=sys.stderr)
    return None

def parse(tarinfo, tar):
    '''
    parse name (bs and thread number)
    and following in kBytes/sec
        Initial write
        Rewrite
        Read
        Re-read
        Random read
        Random write
    '''
    res=[]
    s = NAME_PATTERN.search(tarinfo.name)
    if s is not None:
        res.append(int(s.group(1))) #bs
        res.append(int(s.group(2))) #thread
    try:
        reader = tar.extractfile(tarinfo)
        for line in reader:
            m = KBS_PATTERN.match(line.decode('utf-8'))
            if m is not None:
                res.append(float(m.group(1))/1024.)
    except tarfile.TarError as e:
        print('tar file error:', e, file=sys.stderr)
    return res

def get_data(file_prefix, **args):
    data = []
    for i in range(1,4):
        res = download_untar('{}/{}{}.tar.gz'.format(URL, file_prefix, i))
        if res is None:
            continue
        assert len(res) == 2
        df = pd.DataFrame([parse(r, res[1]) for r in res[0]], columns=COLUMNS)
        data.append(df)
        res[1].close()
        del df
    if len(data) == 0:
        print('Error no input data', file=sys.stderr)
        return None
    df = np.round(pd.concat(data).groupby(level=0).mean(), 2)
    return df

def plot_sparate(df, metrix, name):
    pp = pd.pivot_table(df, index='bs', columns='thread', values=metrix)
    title='{} {} perftest'.format(name, metrix)
    ax = pp.plot.bar(title=title, figsize=(10, 5), rot=0)
    ax.set_ylabel('Throughout MB/s')
    ax.set_xlabel('Block size (KB)')
    plt.tight_layout()
    if not os.path.exists(name):
        os.mkdir(name)
    plt.savefig(os.path.join(name, title+".png"), dpi=128)

def merge_process(zfiles):
    data = []
    for zfile in zfiles:
        res = download_untar('{}/{}'.format(URL ,zfile))
        if res is None:
            continue
        assert len(res) == 2
        df = pd.DataFrame([parse(r, res[1]) for r in res[0]], columns=COLUMNS)
        data.append(df)
        res[1].close()
        del df
    if len(data) == 0:
        print('Error no input data', file=sys.stderr)
        return 1
    df = np.round(pd.concat(data).groupby(level=0).mean(), 2)
    for m in METRIX:
        plot_sparate(df, m, os.path.splitext(zfiles[0])[0])

def plot(df, suffix, verbose):
    '''
    feature: group labels
    https://stackoverflow.com/questions/19184484/how-to-add-group-labels-for-bar-charts-in-matplotlib
    https://github.com/matplotlib/matplotlib/issues/6321
    maximize
    mng = plt.get_current_fig_manager()
    mng.window.showMaximized()
    '''
    title='cephfs vs mfs perftest'
    ax = df.plot.bar(title=title, figsize=(10, 5), rot=0)
    ax.set_ylabel('Throughout MB/s')
    ax.set_xlabel('Block size (KB), Thread')
    plt.xticks(rotation=20)
    plt.tight_layout()
    plt.savefig(title + suffix + ".png", dpi=128)
    if verbose:
        plt.show()

'''
filter thread 4,8 bs 128,512
python plot.py -t 4 -t 8 -s 128 -s 512
'''
def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-v', '--verbose', help='verbose', action='store_true')
    parser.add_argument('-f', '--zfile', help='tgz file name', action='append')
    parser.add_argument('-d', '--file', help='dump data table to file')
    parser.add_argument('-p', '--sparate', help='plot sparately', action='store_true')
    parser.add_argument('-t', '--thread', help='thread filter 1~16', type=int, action='append')
    parser.add_argument('-s', '--bs', help='bs filter 4~2048', type=int, action='append')
    args = parser.parse_args()
    if args.zfile:
        merge_process(args.zfile)
        if args.verbose:
            plt.show()
        return 0

    if args.file:
        import datetime
        now = datetime.datetime.today()
        with open(args.file,'w') as f:
            print('fs', now, file=f)
            print(df.groupby(['bs','thread']).mean()[METRIX], file=f)
            #another way to dump
            #print(pd.pivot_table(df, index=['bs','thread'], values=COLUMNS[2:]))
        return 0

    flter = pd.Series(np.full((df.shape[0],), True))
    suffix = ''
    if not args.thread and not args.bs:
        print('Warning no filter specialed, use default(1024, 8)', file=sys.stderr)
        flter &= ((df['thread']==8) & (df['bs']==1024))
        suffix += ' bs 1024 thread 8'
    else:
        #https://pandas.pydata.org/pandas-docs/stable/indexing.html#indexing-with-isin
        if args.bs:
            if args.verbose:
                print('bs filter', args.bs)
            flter &= df['bs'].isin(args.bs)
            suffix += ' bs {}'.format(' '.join([str(i) for i in args.bs]))
        if args.thread:
            if args.verbose:
                print('thread filter', args.thread)
            flter &= df['thread'].isin(args.thread)
            suffix += ' thread {}'.format(' '.join([str(i) for i in args.thread]))
    df = pd.concat([df1[flter], df2[flter]]).groupby(['fs','bs','thread']).mean()
    df = df[METRIX]
    if args.verbose:
        print(df)
    plot(df, suffix, args.verbose)
    return 0

if __name__ == "__main__":
    sys.exit(main())
