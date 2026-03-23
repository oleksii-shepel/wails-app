declare module 'mp4box' {
  const MP4Box: {
    createFile: (...args: any[]) => any;
    DataStream: new (arrayBuffer?: ArrayBuffer | number, byteOffset?: number, endianness?: boolean) => {
      buffer: ArrayBuffer;
      position: number;
    };
    BoxParser?: any;
  };

  export = MP4Box;
}