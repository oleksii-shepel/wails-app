import { analyzePaneLocations, mirrorPaneLocation, relativePaneLocation, samePaneLocation } from './reassembly-pane-link-policy';

describe('Reassembly pane link policy', () => {
  it('recognizes equivalent Windows locations', () => {
    expect(samePaneLocation('C:\\Work\\Src', 'c:/work/src/')).toBeTrue();
  });

  it('recognizes nested panes', () => {
    expect(analyzePaneLocations('C:/Work/Src/app', 'C:/Work/Src'))
      .toEqual({ relation:'left-inside-right', depthDelta:1 });
  });

  it('mirrors between independent roots', () => {
    expect(mirrorPaneLocation('C:\\src\\app\\assets','C:\\src','D:\\backup'))
      .toBe('D:\\backup\\app\\assets');
  });

  it('mirrors provider locations', () => {
    expect(mirrorPaneLocation('s3://a/project/src','s3://a/project','s3://b/mirror'))
      .toBe('s3://b/mirror/src');
  });

  it('refuses navigation outside the linked root', () => {
    expect(relativePaneLocation('C:/other','C:/src')).toBeNull();
  });
});
