<?xml version="1.0" encoding="utf-8"?>

<xsl:stylesheet version="3.0" 
  xmlns:xsl="http://www.w3.org/1999/XSL/Transform">

  <xsl:mode name="demo" on-no-match="shallow-copy"/>

  <xsl:template match="/">
      <xsl:apply-templates select="root" mode="demo"/>
  </xsl:template>

  <xsl:template match="name" mode="demo">
    <project>
      <xsl:value-of select="upper-case(.)"/>
    </project>
  </xsl:template>

  <xsl:template match="star" mode="demo"/>

</xsl:stylesheet>