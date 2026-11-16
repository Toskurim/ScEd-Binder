# Crea l'icona .ico (formati compatibili) e l'oggetto risorse .syso per Windows amd64, senza strumenti esterni.
import struct, io
from PIL import Image
src=Image.open('preview.png').convert('RGBA')
big=Image.open('icon256.png').convert('RGBA')
sizes=[16,24,32,48,64,256]
imgs=[]
for s in sizes:
    im=(big if s==256 else src.resize((s,s),Image.LANCZOS))
    if s==256:
        b=io.BytesIO(); im.save(b,'PNG'); data=b.getvalue()
    else:
        px=im.load(); rows=b''
        for y in range(s-1,-1,-1):
            rows+=b''.join(bytes((px[x,y][2],px[x,y][1],px[x,y][0],px[x,y][3])) for x in range(s))
        maskrow=b'\x00'*(((s+31)//32)*4)
        data=struct.pack('<IiiHHIIiiII',40,s,s*2,1,32,0,len(rows)+len(maskrow)*s,0,0,0,0)+rows+maskrow*s
    imgs.append((s,data))
# file .ico
ico=struct.pack('<HHH',0,1,len(imgs)); off=6+16*len(imgs)
for s,d in imgs:
    ico+=struct.pack('<BBBBHHII',s%256,s%256,0,0,1,32,len(d),off); off+=len(d)
ico+=b''.join(d for _,d in imgs)
open('sced.ico','wb').write(ico)
# risorse: RT_ICON (3) id 1..n, RT_GROUP_ICON (14) id 1
grp=struct.pack('<HHH',0,1,len(imgs))
for i,(s,d) in enumerate(imgs): grp+=struct.pack('<BBBBHHIH',s%256,s%256,0,0,1,32,len(d),i+1)
n=len(imgs)
def dirhdr(nid): return struct.pack('<IIHHHH',0,0,0,0,0,nid)
# layout offsets
o_root=0; o_l2icon=16+2*8; o_l2grp=o_l2icon+16+n*8; o_l3=o_l2grp+16+8
o_l3icon=[o_l3+i*24 for i in range(n)]; o_l3grp=o_l3+n*24
o_data=o_l3grp+24
o_dicon=[o_data+i*16 for i in range(n)]; o_dgrp=o_data+n*16
o_blob=o_dgrp+16
blobs=[]; pos=o_blob
for s,d in imgs:
    blobs.append((pos,d)); pos+=(len(d)+7)//8*8
blobs.append((pos,grp)); pos+=(len(grp)+7)//8*8
sec=bytearray(pos)
def put(o,b): sec[o:o+len(b)]=b
SUB=0x80000000
put(o_root,dirhdr(2)+struct.pack('<II',3,SUB|o_l2icon)+struct.pack('<II',14,SUB|o_l2grp))
put(o_l2icon,dirhdr(n)+b''.join(struct.pack('<II',i+1,SUB|o_l3icon[i]) for i in range(n)))
put(o_l2grp,dirhdr(1)+struct.pack('<II',1,SUB|o_l3grp))
relocs=[]
for i in range(n):
    put(o_l3icon[i],dirhdr(1)+struct.pack('<II',0x409,o_dicon[i]))
    put(o_dicon[i],struct.pack('<IIII',blobs[i][0],len(blobs[i][1]),0,0)); relocs.append(o_dicon[i])
put(o_l3grp,dirhdr(1)+struct.pack('<II',0x409,o_dgrp))
put(o_dgrp,struct.pack('<IIII',blobs[n][0],len(grp),0,0)); relocs.append(o_dgrp)
for o,d in blobs: put(o,d)
# oggetto COFF
raw_off=20+40; rel_off=raw_off+len(sec); sym_off=rel_off+10*len(relocs)
hdr=struct.pack('<HHIIIHH',0x8664,1,0,sym_off,1,0,0)
sh=b'.rsrc\0\0\0'+struct.pack('<IIIIIIHHI',0,0,len(sec),raw_off,rel_off,0,len(relocs),0,0x40000040)
rel=b''.join(struct.pack('<IIH',o,0,3) for o in relocs)
sym=b'.rsrc\0\0\0'+struct.pack('<IhHBB',0,1,0,3,0)
open('rsrc_windows_amd64.syso','wb').write(hdr+sh+bytes(sec)+rel+sym+struct.pack('<I',4))
print('ico',len(ico),'syso',len(sec),'relocs',len(relocs))
