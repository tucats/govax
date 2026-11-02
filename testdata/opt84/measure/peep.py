import re,sys
INV={'BEQL':'BNEQ','BNEQ':'BEQL','BGTR':'BLEQ','BLEQ':'BGTR','BGEQ':'BLSS','BLSS':'BGEQ','BLBS':'BLBC','BLBC':'BLBS','BGTRU':'BLEQU','BLEQU':'BGTRU','BGEQU':'BLSSU','BLSSU':'BGEQU','BVS':'BVC','BVC':'BVS','BCS':'BCC','BCC':'BCS'}
ins=re.compile(r'^\t([A-Z][A-Z0-9]*)\t?([^;]*?)\s*(;.*)?$')
lab=re.compile(r'^([A-Za-z0-9_$.]+):')
def osize(a):
    a=a.strip(); s=0
    if a.endswith(']'): s+=1; a=a[:a.rindex('[')]
    if re.fullmatch(r'R\d+',a): return s+1
    if a.startswith('#'): return s+5
    if a.startswith('G^'): return s+5
    if re.fullmatch(r'\(R\d+\)',a): return s+1
    return s+5 if '(' not in a else s+5
def size(line):
    m=ins.match(line)
    if m:
        ops=[x for x in split(m.group(2))]
        return 2+sum(osize(x) for x in ops)
    if line.startswith('\t.WORD'): return 2
    if re.match(r'^[^;\s]*\t\.ENTRY',line): return 2
    return 0
def split(s):
    out=[];d=0;c=''
    for ch in s:
        if ch in '(<[': d+=1
        if ch in ')>]': d-=1
        if ch==',' and d==0: out.append(c);c=''
        else: c+=ch
    if c.strip(): out.append(c)
    return out
def run(lines):
    # find code lines indices skipping comments
    changed=True
    while changed:
        changed=False
        pos={};off=0;offs=[]
        for i,l in enumerate(lines):
            offs.append(off)
            m=lab.match(l)
            if m: pos[m.group(1)]=off
            off+=size(l)
        code=[i for i,l in enumerate(lines) if ins.match(l)]
        for k,i in enumerate(code):
            m=ins.match(lines[i]); op,args=m.group(1),m.group(2)
            if op=='BRW' and pos.get(args) is not None and abs(pos[args]-offs[i])<110:
                lines[i]=lines[i].replace('BRW','BRB',1); changed=True; break
            if op in INV and k+1<len(code):
                j=code[k+1]; m2=ins.match(lines[j])
                if m2 and m2.group(1)=='BRW':
                    tgt=args.split(',')[-1].strip(); far=m2.group(2)
                    # the label after the BRW must be op's target
                    labs=[lab.match(x).group(1) for x in lines[j+1:(code[k+2] if k+2<len(code) else len(lines))] if lab.match(x)]
                    if tgt in labs and far in pos and abs(pos[far]-offs[i])<110:
                        pre=code[k-1] if k>0 else None
                        p=ins.match(lines[pre]) if pre is not None else None
                        between=[x for x in lines[pre+1:i] if lab.match(x)] if pre is not None else ['x']
                        if op=='BLEQ' and p and p.group(1)=='DECL' and not between:
                            lines[pre]='\tSOBGTR\t%s,%s'%(p.group(2),far)
                            lines[i]=None; lines[j]=None
                        else:
                            prefix=args.split(',')[:-1]
                            lines[i]='\t%s\t%s'%(INV[op],','.join(prefix+[far]))
                            lines[j]=None
                        lines=[x for x in lines if x is not None]
                        changed=True; break
    return lines
for f in sys.argv[1:]:
    t=open(f,errors='replace').read().split('\n')
    t=run(t)
    open(f.rsplit('/',1)[-1] if 0 else f,'w').write('\n'.join(t))
