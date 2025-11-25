<?xml version="1.0" encoding="utf-8"?>

<xsl:stylesheet version="3.0" 
  xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
  <xsl:mode name="demo" on-no-match="deep-skip"/>

    <xsl:template match="/">
      <element>
        <xsl:apply-templates select="root" mode="demo"/>
      </element>
    </xsl:template>

    <xsl:template match="name" mode="demo">
      <label>
        <xsl:value-of select="."/>
      </label>
    </xsl:template>

    <xsl:template match="root" mode="demo">
      <xsl:apply-templates select="*" mode="demo"/>
    </xsl:template>

</xsl:stylesheet>